// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	samplescheme "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"
	clusterlisters "github.com/m3db/m3db-operator/pkg/client/listers/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/annotations"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	m3placement "github.com/m3db/m3/src/cluster/placement"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"

	jsonpatch "github.com/evanphx/json-patch"
	pkgerrors "github.com/pkg/errors"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

const (
	controllerName       = "m3db-controller"
	clusterWorkQueueName = "m3dbcluster-work-queue"
	podWorkQueueName     = "pods-work-queue"
)

var (
	errOrphanedPod         = errors.New("pod does not belong to an m3db cluster")
	errInvalidNumIsoGroups = errors.New("number of isolationgroups not equal to replication factor")
	errNonUniqueIsoGroups  = errors.New("isolation group names are not unique")
)

type controllerBase struct {
	lock   *sync.Mutex
	logger *zap.Logger
	clock  clock.Clock
	scope  tally.Scope
	doneCh chan struct{}

	adminClient *multiAdminClient

	kubeClient kubernetes.Interface
	crdClient  clientset.Interface

	statefulSetLister  appslisters.StatefulSetLister
	statefulSetsSynced cache.InformerSynced
	podLister          corelisters.PodLister
	podsSynced         cache.InformerSynced

	podWorkQueue workqueue.RateLimitingInterface

	recorder eventer.Poster
}

// M3DBController object
type M3DBController struct {
	controllerBase

	k8sclient     m3db.K8sops
	podIDProvider podidentity.Provider

	clusterLister  clusterlisters.M3DBClusterLister
	clustersSynced cache.InformerSynced

	clusterWorkQueue workqueue.RateLimitingInterface
}

// NewM3DBController creates new instance of Controller
func NewM3DBController(opts ...Option) (*M3DBController, error) {
	options := &options{}

	for _, o := range opts {
		o.execute(options)
	}

	if err := options.validate(); err != nil {
		return nil, err
	}

	kclient := options.kclient
	kubeClient := options.kubeClient
	crdClient := options.crdClient
	scope := options.scope

	logger := options.logger
	if logger == nil {
		logger = zap.NewNop()
	}

	adminOpts := []m3admin.Option{m3admin.WithLogger(logger)}
	multiClient := newMultiAdminClient(adminOpts, logger)
	if options.kubectlProxy {
		multiClient.clusterURLFn = clusterURLProxy
	}

	statefulSetInformer := options.filteredInformerFactory.Apps().V1().StatefulSets()
	podInformer := options.filteredInformerFactory.Core().V1().Pods()
	m3dbClusterInformer := options.m3dbClusterInformerFactory.Operator().V1alpha1().M3DBClusters()

	samplescheme.AddToScheme(scheme.Scheme)

	clusterWorkQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), clusterWorkQueueName)
	podWorkQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), podWorkQueueName)

	r, err := eventer.NewEventRecorder(eventer.WithClient(kubeClient), eventer.WithLogger(logger), eventer.WithComponent(controllerName))
	if err != nil {
		return nil, err
	}

	p := &M3DBController{
		controllerBase: controllerBase{
			lock:   &sync.Mutex{},
			logger: logger,
			scope:  scope,
			clock:  clock.RealClock{},

			adminClient: multiClient,
			doneCh:      make(chan struct{}),

			kubeClient: kubeClient,
			crdClient:  crdClient,

			statefulSetLister:  statefulSetInformer.Lister(),
			statefulSetsSynced: statefulSetInformer.Informer().HasSynced,
			podLister:          podInformer.Lister(),
			podsSynced:         podInformer.Informer().HasSynced,

			podWorkQueue: podWorkQueue,
			// TODO(celina): figure out if we actually need a recorder for each namespace
			recorder: r,
		},

		k8sclient:     kclient,
		podIDProvider: options.podIDProvider,

		clusterLister:  m3dbClusterInformer.Lister(),
		clustersSynced: m3dbClusterInformer.Informer().HasSynced,

		clusterWorkQueue: clusterWorkQueue,
	}

	m3dbClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: p.enqueueCluster,
		UpdateFunc: func(old, new interface{}) {
			p.enqueueCluster(new)
		},
		DeleteFunc: func(obj interface{}) {
			// TODO(schallert): what do we want to do on delete? clean up the etcd
			// data? we don't need to do anything w/ the sts + pods, we set owner refs
			// so kubernetes will GC them for us
			logger.Info("deleted cluster")
		},
	})

	statefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: p.handleStatefulSetUpdate,
		UpdateFunc: func(old, new interface{}) {
			oldSts := old.(*appsv1.StatefulSet)
			newSts := new.(*appsv1.StatefulSet)

			// TODO(schallert): version vs. observed generation?
			if newSts.ResourceVersion == oldSts.ResourceVersion {
				// don't have to reprocess on periodic resync
				return
			}

			p.handleStatefulSetUpdate(new)
		},
		DeleteFunc: p.handleStatefulSetUpdate,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: p.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			p.enqueuePod(new)
		},
		DeleteFunc: func(obj interface{}) {
			// No-op
		},
	})

	return p, nil
}

// Run drives the controller event loop.
func (c *M3DBController) Run(nWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.clusterWorkQueue.ShutDown()

	c.logger.Info("starting Operator controller")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.logger.Info("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.clustersSynced, c.statefulSetsSynced, c.podsSynced); !ok {
		return errors.New("caches failed to sync")
	}

	c.logger.Info("starting workers")
	for i := 0; i < nWorkers; i++ {
		go c.runClusterLoop(ctx)
		go c.runPodLoop(ctx)
	}

	c.logger.Info("workers started")
	<-stopCh
	c.logger.Info("shutting down workers")

	return nil
}

func (c *M3DBController) enqueueCluster(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.clusterWorkQueue.AddRateLimited(key)
	c.scope.Counter("enqueued_event").Inc(int64(1))
}

func (c *M3DBController) runClusterLoop(ctx context.Context) {
	for c.processClusterQueueItem(ctx) {
	}
}

//nolint:dupl
func (c *M3DBController) processClusterQueueItem(ctx context.Context) bool {
	obj, shutdown := c.clusterWorkQueue.Get()
	c.scope.Counter("dequeued_event").Inc(int64(1))
	if shutdown {
		return false
	}

	// Closure so we can defer workQueue.Done.
	err := func(obj interface{}) error {
		defer c.clusterWorkQueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.clusterWorkQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string from queue, got %#v", obj))
			return nil
		}

		if err := c.handleClusterEvent(ctx, key); err != nil {
			return fmt.Errorf("error syncing cluster '%s': %v", key, err)
		}

		c.clusterWorkQueue.Forget(obj)
		c.logger.Info("successfully synced item", zap.String("key", key))

		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
	}

	return true
}

func (c *M3DBController) handleClusterEvent(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	cluster, err := c.clusterLister.M3DBClusters(namespace).Get(name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// give up processing if this cluster doesn't exist
			runtime.HandleError(fmt.Errorf("clusters '%s' no longer exists", key))
			return nil
		}

		return err
	}

	if cluster == nil {
		return errors.New("got nil cluster for " + key)
	}

	return c.handleClusterUpdate(ctx, cluster)
}

// We are guaranteed by handleClusterEvent that we will never be passed a nil
// cluster here.
func (c *M3DBController) handleClusterUpdate(
	ctx context.Context, cluster *myspec.M3DBCluster,
) error {
	// MUST create a deep copy of the cluster or risk corrupting cache! Technically
	// only need if we modify, but we frequently do that so let's deep copy to
	// start and remove unnecessary calls later to optimize if we want.
	cluster = cluster.DeepCopy()

	clusterLogger := c.logger.With(
		zap.String("cluster", cluster.Name),
		zap.String("namespace", cluster.Namespace),
	)

	if cluster.Spec.Frozen {
		clusterLogger.Info("cluster is frozen so no changes will be made")
		return nil
	}

	// https://v1-12.docs.kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/
	//
	// If deletion timestamp is zero (cluster hasn't been deleted), make sure our
	// finalizer is present. If the cluster has been marked for deletion, delete
	// the placement and namespace.
	if dts := cluster.ObjectMeta.DeletionTimestamp; dts != nil && !dts.IsZero() {
		if !stringArrayContains(cluster.Finalizers, labels.EtcdDeletionFinalizer) {
			clusterLogger.Info("no etcd finalizer on cluster, nothing to do")
			return nil
		}

		// If cluster is set to preserve data, jump straight to removing the
		// finalizer.
		if cluster.Spec.KeepEtcdDataOnDelete {
			clusterLogger.Info("skipping etcd deletion due to keepEtcdDataOnDelete")
		} else {
			if err := c.deleteAllNamespaces(cluster); err != nil {
				clusterLogger.Error("error deleting cluster namespaces", zap.Error(err))
				return err
			}

			if err := c.deletePlacement(cluster); err != nil {
				clusterLogger.Error("error deleting cluster placement", zap.Error(err))
				return err
			}
		}

		if _, err := c.removeEtcdFinalizer(ctx, cluster); err != nil {
			clusterLogger.Error("error deleting etcd finalizer", zap.Error(err))
			return pkgerrors.WithMessage(err, "error removing etcd cluster finalizer")
		}

		// Exit the control loop once the cluster is deleted and cleaned up.
		clusterLogger.Info("completed finalizer cleanup")
		return nil
	}

	if err := validateIsolationGroups(cluster); err != nil {
		clusterLogger.Error("failed validating isolationgroups", zap.Error(err))
		c.recorder.WarningEvent(cluster, eventer.ReasonFailSync, err.Error())
		return err
	}

	if !cluster.Spec.KeepEtcdDataOnDelete {
		var err error
		cluster, err = c.ensureEtcdFinalizer(ctx, cluster)
		if err != nil {
			return err
		}
	}

	if err := c.ensureConfigMap(ctx, cluster); err != nil {
		clusterLogger.Error("failed to ensure configmap", zap.Error(err))
		c.recorder.WarningEvent(cluster, eventer.ReasonFailSync, "failed to ensure configmap: %s", err.Error())
		return err
	}

	// Per https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#statefulsetspec-v1-apps,
	// headless service MUST exist before statefulset.
	if err := c.ensureServices(ctx, cluster); err != nil {
		return err
	}

	if len(cluster.Spec.IsolationGroups) == 0 {
		// nothing to do, no groups to create in
		return nil
	}

	// copy since we sort the array
	isoGroups := make([]myspec.IsolationGroup, len(cluster.Spec.IsolationGroups))
	copy(isoGroups, cluster.Spec.IsolationGroups)
	sort.Sort(myspec.IsolationGroups(isoGroups))

	childrenSets, err := c.getChildStatefulSets(cluster)
	if err != nil {
		return err
	}

	childrenSetsByName := make(map[string]*appsv1.StatefulSet)
	for _, sts := range childrenSets {
		childrenSetsByName[sts.Name] = sts
	}

	// Create any missing statefulsets, at this point all existing stateful sets are bootstrapped.
	for i := 0; i < len(isoGroups); i++ {
		name := m3db.StatefulSetName(cluster.Name, i)
		_, exists := childrenSetsByName[name]
		if !exists {
			sts, err := m3db.GenerateStatefulSet(cluster, isoGroups[i].Name, isoGroups[i].NumInstances)
			if err != nil {
				return err
			}

			_, err = c.kubeClient.AppsV1().StatefulSets(cluster.Namespace).
				Create(ctx, sts, metav1.CreateOptions{})
			if err != nil {
				if kerrors.IsAlreadyExists(err) {
					// There may be a delay between when a StatefulSet is created and when it is
					// returned in calls to List because of the caching that the k8s client does.
					// So if we receive an error because the StatefulSet already exists that's not
					// indicative of a real problem.
					c.logger.Info("statefulset already exists", zap.String("name", name))
					return nil
				}
				c.logger.Error(err.Error())
				return err
			}

			c.logger.Info("created statefulset", zap.String("name", name))
			return nil
		}
	}

	// For all statefulsets, ensure their observered generation is up to date.
	// This means that the k8s statefulset controller has updated Status (and
	// therefore ready replicas + updated replicas). If observed generation !=
	// generation, it means Status will contain stale info.
	for _, sts := range childrenSets {
		if sts.Generation != sts.Status.ObservedGeneration {
			c.logger.Warn("stateful set not up to date",
				zap.String("namespace", sts.Namespace),
				zap.String("name", sts.Name),
				zap.Int32("readyReplicas", sts.Status.ReadyReplicas),
				zap.Int32("updatedReplicas", sts.Status.UpdatedReplicas),
				zap.String("currentRevision", sts.Status.CurrentRevision),
				zap.String("updateRevision", sts.Status.UpdateRevision),
				zap.Int64("generation", sts.Generation),
				zap.Int64("observed", sts.Status.ObservedGeneration),
			)
			return fmt.Errorf("set %s generation is not up to date (current: %d, observed: %d)", sts.Name, sts.Generation, sts.Status.ObservedGeneration)
		}
	}

	// If any of the statefulsets aren't ready, wait until they are as we'll get
	// another event (ready == bootstrapped)
	for _, sts := range childrenSets {
		c.logger.Debug("processing set",
			zap.String("namespace", sts.Namespace),
			zap.String("name", sts.Name),
			zap.Int32("readyReplicas", sts.Status.ReadyReplicas),
			zap.Int32("updatedReplicas", sts.Status.UpdatedReplicas),
			zap.String("currentRevision", sts.Status.CurrentRevision),
			zap.String("updateRevision", sts.Status.UpdateRevision),
			zap.Int64("generation", sts.Generation),
			zap.Int64("observed", sts.Status.ObservedGeneration),
		)

		if sts.Spec.Replicas == nil {
			c.logger.Warn("skip check for statefulset, replicas is nil",
				zap.String("name", sts.Name),
				zap.Int32("readyReplicas", sts.Status.ReadyReplicas),
				zap.Int32("updatedReplicas", sts.Status.UpdatedReplicas),
				zap.String("currentRevision", sts.Status.CurrentRevision),
				zap.String("updateRevision", sts.Status.UpdateRevision))
			continue
		}

		replicas := *sts.Spec.Replicas
		ready := replicas == sts.Status.ReadyReplicas
		if sts.Status.UpdateRevision != "" &&
			sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType {
			// If there is an update revision, ensure all pods are updated
			// otherwise there is a rollout in progress.
			// Note: This ensures there is no race condition between
			// updating a stateful set and it seemingly being "ready" and
			// all pods healthy immediately after the update, since the
			// updated replicas will not match the desired replicas
			// and avoid the race condition of proceeding to another update
			// before a stateful set has had a chance to update the pods
			// but otherwise seemingly is healthy.
			ready = ready && replicas == sts.Status.UpdatedReplicas
		}

		if !ready {
			c.logger.Info("waiting for statefulset to be ready",
				zap.String("namespace", sts.Namespace),
				zap.String("name", sts.Name),
				zap.Int32("replicas", replicas),
				zap.Int32("readyReplicas", sts.Status.ReadyReplicas),
				zap.Int32("updatedReplicas", sts.Status.UpdatedReplicas),
				zap.String("currentRevision", sts.Status.CurrentRevision),
				zap.String("updateRevision", sts.Status.UpdateRevision))
			return nil
		}
	}

	// Now that we know the cluster is healthy, iterate over each isolation group and check
	// if it should be updated.
	for i := 0; i < len(isoGroups); i++ {
		name := m3db.StatefulSetName(cluster.Name, i)

		// This StatefulSet is guaranteed to exist since if it didn't originally it would be
		// created above when we first iterate over the isolation groups.
		actual := childrenSetsByName[name]
		expected, update, err := updatedStatefulSet(actual, cluster, isoGroups[i])
		if err != nil {
			c.logger.Error(err.Error())
			return err
		}

		// If we're not updating the statefulset AND we're not using the OnDelete update
		// strategy, then move to the next statefulset. When using the OnDelete update
		// strategy, we still may want to restart nodes for this particular statefulset,
		// so don't continue yet.
		onDeleteUpdateStrategy := actual.Spec.UpdateStrategy.Type ==
			appsv1.OnDeleteStatefulSetStrategyType

		if !update && !onDeleteUpdateStrategy {
			continue
		}

		if update {
			_, err = c.applyStatefulSetUpdate(ctx, cluster, actual, expected)
			if err != nil {
				c.logger.Error(err.Error())
				return err
			}
			return nil
		}

		// Using an OnDelete strategy, we have to update nodes if:
		//   - a statefulset update has happened OR
		//   - we're already in the middle of a rollout
		//     * because nodes are rolled out in chunks, this can happen over many iterations
		// Therefore, check to see if pods need to be updated and return from this loop
		// if pods were updated. If a rollout is finished or there has not
		// been a change, this call is a no-op.
		if onDeleteUpdateStrategy {
			nodesUpdated, err := c.updateStatefulSetPods(ctx, cluster, actual)
			if err != nil {
				c.logger.Error("error performing update",
					zap.Error(err),
					zap.String("namespace", cluster.Namespace),
					zap.String("name", actual.Name))
				return err
			}

			// If we've performed any updates at all, do not process the next statefulset.
			// Wait for the updated pods to become healthy.
			if nodesUpdated {
				return nil
			}
		}
	}

	if !cluster.Status.HasInitializedPlacement() {
		cluster, err = c.validatePlacementWithStatus(ctx, cluster)
		if err != nil {
			return err
		}
	}

	if err := c.reconcileNamespaces(cluster); err != nil {
		c.recorder.WarningEvent(cluster, eventer.ReasonFailedCreate, "failed to create namespace: %s", err)
		c.logger.Error("error reconciling namespaces", zap.Error(err))
		return err
	}

	if len(cluster.Spec.Namespaces) == 0 {
		c.logger.Warn("cluster has no namespaces defined", zap.String("cluster", cluster.Name))
		c.recorder.WarningEvent(cluster, eventer.ReasonUnknown, "cluster %s has no namespaces", cluster.Name)
	}

	// At this point we have the desired number of statefulsets, and every pod
	// across those sets is bootstrapped. However some may be bootstrapped because
	// they own no shards. Check to see that all pods are in the placement.
	clusterPodsSelector := klabels.SelectorFromSet(labels.BaseLabels(cluster))
	pods, err := c.podLister.Pods(cluster.Namespace).List(clusterPodsSelector)
	if err != nil {
		return fmt.Errorf("error listing pods to construct placement: %v", err)
	}

	placement, err := c.adminClient.placementClientForCluster(cluster).Get()
	if err != nil {
		return fmt.Errorf("error fetching active placement: %v", err)
	}

	c.logger.Info("found placement", zap.Int("currentPods", len(pods)), zap.Int("placementInsts", placement.NumInstances()))

	unavailInsts := []string{}
	for _, inst := range placement.Instances() {
		if !inst.IsAvailable() {
			unavailInsts = append(unavailInsts, inst.ID())
		}
	}

	if ln := len(unavailInsts); ln > 0 {
		c.logger.Warn("waiting for instances to be available", zap.Strings("instances", unavailInsts))
		c.recorder.WarningEvent(cluster, eventer.ReasonLongerThanUsual, "current unavailable instances: %d", ln)
		return nil
	}

	// Determine if any sets aren't at their desired replica count. Maybe we can
	// reuse the set objects from above but being paranoid for now.
	childrenSets, err = c.getChildStatefulSets(cluster)
	if err != nil {
		return err
	}

	// check if any pods inside the cluster need to be swapped in
	leavingInstanceID, podToReplace, err := c.checkPodsForReplacement(cluster, pods, placement)
	if err != nil {
		return err
	}

	if podToReplace != nil {
		err = c.replacePodInPlacement(ctx, cluster, placement, leavingInstanceID, podToReplace)
		if err != nil {
			c.recorder.WarningEvent(cluster, eventer.ReasonFailedToUpdate, "could not replace instance: "+leavingInstanceID)
			return err
		}
		c.recorder.NormalEvent(cluster, eventer.ReasonSuccessfulUpdate, "successfully replaced instance: "+leavingInstanceID)
	}

	for _, set := range childrenSets {
		zone, ok := set.Labels[labels.IsolationGroup]
		if !ok {
			return fmt.Errorf("statefulset %s has no isolation-group label", set.Name)
		}

		group, ok := myspec.IsolationGroups(isoGroups).GetByName(zone)
		if !ok {
			return fmt.Errorf("zone %s not found in cluster isoGroups %v", zone, isoGroups)
		}

		if set.Spec.Replicas == nil {
			return fmt.Errorf("set %s has unset spec replica", set.Name)
		}

		// NB(cerkauskas): if statefulset is managed using on delete strategy, then operator
		// should not expand nor shrink the cluster without the annotation
		if set.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
			_, inProgressAnnotationExists := set.Annotations[annotations.ParallelUpdateInProgress]
			if !inProgressAnnotationExists {
				c.logger.Warn("skipping statefulset resize because it does not have progress annotation", zap.String("sts", set.Name))
				continue
			}
		}

		// Number of pods we want in the group.
		desired := group.NumInstances
		// Number of pods currently in the group.
		current := *set.Spec.Replicas
		// Number of instances in the group AND currently in the placement.
		inPlacement := int32(len(instancesInIsoGroup(placement, group.Name)))

		setLogger := c.logger.With(
			zap.String("statefulSet", set.Name),
			zap.Int32("inPlacement", inPlacement),
			zap.Int32("current", current),
			zap.Int32("desired", desired),
		)

		if desired == current {
			// If the set is at its desired size, and all pods in the set are in the
			// placement, there's nothing we need to do for this set.
			if current == inPlacement {
				continue
			}

			// If the set is at its desired size but there's pods in the set that are
			// absent from the placement, add pods to placement.
			if inPlacement < current {
				setLogger.Info("expanding placement for set")
				return c.expandPlacementForSet(ctx, cluster, set, group, placement)
			}
		}

		// If there are more pods in the placement than we want in the group,
		// trigger a remove so that we can shrink the set.
		if inPlacement > desired {
			setLogger.Info("shrinking placement for set")
			return c.shrinkPlacementForSet(cluster, set, placement, int(desired))
		}

		var newCount int32
		if current < desired {
			newCount = current + 1
		} else {
			if cluster.Spec.PreventScaleDown {
				return pkgerrors.Errorf("cannot shrink statefulset %s/%s, preventScaleDown is true",
					set.Namespace, set.Name)
			}
			newCount = current - 1
		}
		setLogger.Info("resizing set, desired != current", zap.Int32("newSize", newCount))

		if err = c.patchStatefulSet(ctx, set, func(set *appsv1.StatefulSet) {
			set.Spec.Replicas = pointer.Int32Ptr(newCount)
		}); err != nil {
			c.logger.Error("error patching statefulset", zap.Error(err))
			return err
		}

		return nil
	}

	placement, err = c.adminClient.placementClientForCluster(cluster).Get()
	if err != nil {
		return fmt.Errorf("error fetching placement: %v", err)
	}

	// TODO(celina): possibly do a replacement check here

	// See if we need to clean up the pod bootstrapping status.
	cluster, err = c.reconcileBootstrappingStatus(ctx, cluster, placement)
	if err != nil {
		return fmt.Errorf("error reconciling bootstrap status: %v", err)
	}

	err = c.cleanupAnnotations(ctx, childrenSets)
	if err != nil {
		return fmt.Errorf("error cleaning up annotations for on delete strategy: %w", err)
	}

	c.logger.Info("nothing to do",
		zap.Int("childrensets", len(childrenSets)),
		zap.Int("zones", len(isoGroups)),
		zap.Int64("generation", cluster.ObjectMeta.Generation),
		zap.String("rv", cluster.ObjectMeta.ResourceVersion))

	return nil
}

func (c *M3DBController) cleanupAnnotations(
	ctx context.Context, childrenSets []*appsv1.StatefulSet,
) error {
	c.logger.Debug("cleaning up progress annotations")
	for _, set := range childrenSets {
		// NB(cerkauskas): we want to delete annotation no matter the strategy of cluster.
		// Strategy could be changed while update is happening and we still want to remove
		// the annotation since there is nothing more to do for operator.
		if _, ok := set.Annotations[annotations.ParallelUpdateInProgress]; !ok {
			c.logger.Debug("skipping set because it does not have progress annotation",
				zap.String("sts", set.Name))
			continue
		}

		c.logger.Info("removing update annotation for statefulset",
			zap.String("sts", set.Name))

		if err := c.patchStatefulSet(ctx, set, func(set *appsv1.StatefulSet) {
			delete(set.Annotations, annotations.ParallelUpdateInProgress)
		}); err != nil {
			c.logger.Error("failed to remove annotation",
				zap.String("sts", set.Name),
				zap.Error(err))
			return err
		}
	}
	c.logger.Info("cleaned up progress annotations")
	return nil
}

func (c *M3DBController) patchStatefulSet(
	ctx context.Context,
	set *appsv1.StatefulSet,
	action func(set *appsv1.StatefulSet),
) error {
	setBytes, err := json.Marshal(set)
	if err != nil {
		return err
	}

	action(set)

	setModifiedBytes, err := json.Marshal(set)
	if err != nil {
		return err
	}

	patchBytes, err := jsonpatch.CreateMergePatch(setBytes, setModifiedBytes)
	if err != nil {
		return err
	}

	set, err = c.kubeClient.
		AppsV1().
		StatefulSets(set.Namespace).
		Patch(ctx, set.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error updating statefulset %s: %w", set.Name, err)
	}

	return nil
}

func (c *M3DBController) applyStatefulSetUpdate(
	ctx context.Context,
	cluster *myspec.M3DBCluster,
	actual *appsv1.StatefulSet,
	expected *appsv1.StatefulSet,
) (*appsv1.StatefulSet, error) {
	updated, err := c.kubeClient.AppsV1().StatefulSets(cluster.Namespace).
		Update(ctx, expected, metav1.UpdateOptions{})
	if err != nil {
		c.logger.Error(err.Error())
		return nil, err
	}

	c.logger.Info("updated statefulset",
		zap.String("name", expected.Name),
		zap.Int32("actual_readyReplicas", actual.Status.ReadyReplicas),
		zap.Int32("actual_updatedReplicas", actual.Status.UpdatedReplicas),
		zap.String("actual_currentRevision", actual.Status.CurrentRevision),
		zap.String("actual_updateRevision", actual.Status.UpdateRevision),
		zap.Int32("expected_readyReplicas", expected.Status.ReadyReplicas),
		zap.Int32("expected_updatedReplicas", expected.Status.UpdatedReplicas),
		zap.String("expected_currentRevision", expected.Status.CurrentRevision),
		zap.String("expected_updateRevision", expected.Status.UpdateRevision),
		zap.Int64("generation", expected.Generation),
		zap.Int64("observed", expected.Status.ObservedGeneration),
	)
	return updated, nil
}

// updateStatefulSetPods returns true if it updates any pods
func (c *M3DBController) updateStatefulSetPods(
	ctx context.Context,
	cluster *myspec.M3DBCluster,
	sts *appsv1.StatefulSet,
) (bool, error) {
	logger := c.logger.With(
		zap.String("namespace", cluster.Namespace), zap.String("name", sts.Name),
	)

	if _, ok := sts.Annotations[annotations.ParallelUpdateInProgress]; !ok {
		logger.Debug("no update and no rollout in progress so move to next statefulset")
		return false, nil
	}

	numPods, err := getMaxPodsToUpdate(sts)
	if err != nil {
		return false, err
	}

	if numPods == 0 {
		return false, errors.New("parallel update annotation set to 0. will not perform pod updates")
	}

	pods, err := c.podsToUpdate(cluster.Namespace, sts, numPods)
	if err != nil {
		return false, err
	}

	if len(pods) > 0 {
		names := make([]string, 0, len(pods))
		for _, pod := range pods {
			if err := c.kubeClient.CoreV1().
				Pods(pod.Namespace).
				Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
				return false, err
			}
			names = append(names, pod.Name)
		}
		logger.Info("restarting pods", zap.Strings("pods", names))
		return true, nil
	}

	// If there are no pods to update, we're fully rolled out so remove
	// the update annotation and update status.
	//
	// NB(nate): K8s handles this for you when using the RollingUpdate update strategy.
	// However, since OnDelete removes k8s from the pod update process, it's our
	// responsibility to set this once done rolling out.
	sts.Status.CurrentReplicas = sts.Status.UpdatedReplicas
	sts.Status.CurrentRevision = sts.Status.UpdateRevision

	if _, err = c.kubeClient.AppsV1().StatefulSets(cluster.Namespace).
		UpdateStatus(ctx, sts, metav1.UpdateOptions{}); err != nil {
		return false, err
	}

	logger.Info("update of existing pods complete")

	return false, nil
}

func getMaxPodsToUpdate(actual *appsv1.StatefulSet) (int, error) {
	var (
		rawVal string
		ok     bool
	)
	if rawVal, ok = actual.Annotations[annotations.ParallelUpdateInProgress]; !ok {
		return 0, errors.New("parallel update annotation missing during statefulset update")
	}

	var (
		maxPodsPerUpdate int
		err              error
	)
	if maxPodsPerUpdate, err = strconv.Atoi(rawVal); err != nil {
		return 0, fmt.Errorf("failed to parse parallel update annotation: %v", rawVal)
	}

	return maxPodsPerUpdate, nil
}

func (c *M3DBController) podsToUpdate(
	namespace string,
	sts *appsv1.StatefulSet,
	numPods int,
) ([]*corev1.Pod, error) {
	var (
		currRev   = sts.Status.CurrentRevision
		updateRev = sts.Status.UpdateRevision
	)
	// nolint:gocritic
	if currRev == "" {
		return nil, errors.New("currentRevision empty")
	} else if updateRev == "" {
		return nil, errors.New("updateRevision empty")
	} else if currRev == updateRev {
		// No pods to update because current and update revision are the same
		return nil, nil
	}

	// Get any pods not on the updateRevision for this statefulset
	podSelector, err := generatePodSelector(updateRev, sts)
	if err != nil {
		return nil, err
	}
	pods, err := c.podLister.Pods(namespace).List(podSelector)
	if err != nil {
		return nil, err
	} else if len(pods) == 0 {
		return nil, nil
	}

	// NB(nate): Sort here so updates are always done in a consistent order.
	// Statefulset 0 -> N: Pod 0 -> N
	sortedPods, err := sortPods(pods)
	if err != nil {
		return nil, err
	}

	toUpdate := make([]*corev1.Pod, 0, len(sortedPods))
	for _, pod := range sortedPods {
		toUpdate = append(toUpdate, pod.pod)
		if len(toUpdate) == numPods {
			break
		}
	}

	return toUpdate, nil
}

func generatePodSelector(updateRev string, sts *appsv1.StatefulSet) (klabels.Selector, error) {
	revReq, err := klabels.NewRequirement(
		"controller-revision-hash", selection.NotEquals, []string{updateRev},
	)
	if err != nil {
		return nil, err
	}
	stsReq, err := klabels.NewRequirement(
		labels.StatefulSet, selection.Equals, []string{sts.Name},
	)
	if err != nil {
		return nil, err
	}
	podSelector := klabels.NewSelector().Add(*revReq, *stsReq)
	return podSelector, nil
}

func instancesInIsoGroup(pl m3placement.Placement, isoGroup string) []m3placement.Instance {
	insts := []m3placement.Instance{}
	for _, inst := range pl.Instances() {
		if inst.IsolationGroup() == isoGroup {
			insts = append(insts, inst)
		}
	}
	return insts
}

// This func is currently read-only, but if we end up modifying statefulsets
// we'll have to deepcopy.
func (c *M3DBController) getChildStatefulSets(cluster *myspec.M3DBCluster) ([]*appsv1.StatefulSet, error) {
	labels := labels.BaseLabels(cluster)
	statefulSets, err := c.statefulSetLister.StatefulSets(cluster.Namespace).List(klabels.Set(labels).AsSelector())
	if err != nil {
		runtime.HandleError(fmt.Errorf("error listing statefulsets: %v", err))
		return nil, err
	}

	childrenSets := make([]*appsv1.StatefulSet, 0)
	for _, sts := range statefulSets {
		if metav1.IsControlledBy(sts, cluster) {
			childrenSets = append(childrenSets, sts.DeepCopy())
		}
	}

	return childrenSets, nil
}

func (c *M3DBController) handleStatefulSetUpdate(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}

		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding tombstone, invalid type"))
			return
		}

		c.logger.Info("recovered object from tombstone", zap.String("name", object.GetName()))
	}

	c.logger.Info("processing statefulset", zap.String("statefulset.namespace", object.GetNamespace()), zap.String("statefulset.name", object.GetName()))

	owner := metav1.GetControllerOf(object)
	// TODO(schallert): const
	if owner == nil || owner.Kind != "m3dbcluster" {
		return
	}

	cluster, err := c.clusterLister.M3DBClusters(object.GetNamespace()).Get(owner.Name)
	if err != nil {
		c.logger.Info("ignoring orphaned object", zap.String("m3dbcluster", owner.Name), zap.String("namespace", object.GetNamespace()), zap.String("statefulset", object.GetName()))
		return
	}

	// enqueue the cluster for processing
	c.enqueueCluster(cluster)
}

func (c *M3DBController) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		c.logger.Error("error splitting pod cache key", zap.Error(err))
		return
	}
	c.podWorkQueue.AddRateLimited(key)
	c.scope.Counter("enqueued_event").Inc(int64(1))
}

func (c *M3DBController) runPodLoop(ctx context.Context) {
	for c.processPodQueueItem(ctx) {
	}
}

//nolint:dupl
func (c *M3DBController) processPodQueueItem(ctx context.Context) bool {
	obj, shutdown := c.podWorkQueue.Get()
	c.scope.Counter("dequeued_event").Inc(int64(1))
	if shutdown {
		return false
	}

	// Closure so we can defer workQueue.Done.
	err := func(obj interface{}) error {
		defer c.podWorkQueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.podWorkQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string from queue, got %#v", obj))
			return nil
		}

		if err := c.handlePodEvent(ctx, key); err != nil {
			return fmt.Errorf("error syncing cluster '%s': %v", key, err)
		}

		c.podWorkQueue.Forget(obj)
		c.logger.Debug("successfully synced item", zap.String("key", key))

		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
	}

	return true
}

func (c *M3DBController) handlePodEvent(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.Error("invalid resource key", zap.Error(err))
		return nil
	}

	pod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			c.logger.Debug("pod no longer exists", zap.String("pod", name))
			return nil
		}

		c.logger.Error("error listing pods in pod handle", zap.String("pod", name), zap.Error(err))
		return err
	}

	if pod == nil {
		return errors.New("got nil pod for key " + key)
	}

	return c.handlePodUpdate(ctx, pod)
}

func (c *M3DBController) handlePodUpdate(ctx context.Context, pod *corev1.Pod) error {
	// We only process pods that are members of m3db clusters.
	if _, err := getClusterValue(pod); err != nil {
		return nil
	}

	pod = pod.DeepCopy()

	podBytes, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	podLogger := c.logger.With(zap.String("pod.namespace", pod.Namespace), zap.String("pod.name", pod.Name))
	podLogger.Info("processing pod")

	cluster, err := c.getParentCluster(pod)
	if err != nil {
		podLogger.Error("error getting parent cluster", zap.Error(err))
		return err
	}

	id, err := c.podIDProvider.Identity(pod, cluster)
	if err != nil {
		podLogger.Error("error getting pod ID", zap.Error(err))
		return err
	}

	podLogger.Debug("pod ID", zap.Any("id", id))

	idStr, err := podidentity.IdentityJSON(id)
	if err != nil {
		podLogger.Error("error marshaling pod ID", zap.Error(err))
		return err
	}

	currentID, ok := pod.Annotations[podidentity.AnnotationKeyPodIdentity]
	if ok {
		if currentID != idStr {
			podLogger.Warn("pod ID mismatch",
				zap.String("currentID", currentID),
				zap.String("newID", idStr))
		}

		// TODO(schallert): decide how to enforce updated pod identity (need to
		// determine ramnifications of changing). Will likely need to do a replace.
		return nil
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[podidentity.AnnotationKeyPodIdentity] = idStr

	podModifiedBytes, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	// NB(r): See reference of use in the AddLabelToPod of postgres operator:
	// https://github.com/CrunchyData/postgres-operator/blob/7e867ebf211b488def26dfaf10d7b2b5bbd6f5f6/kubeapi/pod.go#L108
	patchBytes, err := jsonpatch.CreateMergePatch(podBytes, podModifiedBytes)
	if err != nil {
		return err
	}

	_, err = c.kubeClient.CoreV1().
		Pods(pod.Namespace).
		Patch(ctx, pod.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		podLogger.Error("error updating pod annotation", zap.Error(err))
		return err
	}

	podLogger.Info("updated pod ID", zap.Any("id", id))
	c.recorder.NormalEvent(pod, eventer.ReasonSuccessSync, "updated pod %s with ID annotation", pod.Name)

	return nil
}

func getClusterValue(pod *corev1.Pod) (string, error) {
	cluster, ok := pod.Labels[labels.Cluster]
	if !ok {
		return "", errOrphanedPod
	}

	return cluster, nil
}

func (c *M3DBController) getParentCluster(pod *corev1.Pod) (*myspec.M3DBCluster, error) {
	clusterName, err := getClusterValue(pod)
	if err != nil {
		return nil, err
	}

	cluster, err := c.clusterLister.M3DBClusters(pod.Namespace).Get(clusterName)
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func validateIsolationGroups(cluster *myspec.M3DBCluster) error {
	groups := cluster.Spec.IsolationGroups

	if cluster.Spec.ReplicationFactor != int32(len(groups)) {
		return pkgerrors.WithMessagef(errInvalidNumIsoGroups, "replication factor is %d but number of isogroups is %d", cluster.Spec.ReplicationFactor, len(groups))
	}

	names := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		names[g.Name] = struct{}{}
	}

	if len(names) != len(groups) {
		return pkgerrors.WithMessagef(errNonUniqueIsoGroups, "found %d isolationGroups but %d unique names", len(groups), len(names))
	}
	return nil
}

// updatedStatefulSet checks if we should update the actual StatefulSet to match it's
// expected state. If so, it returns true and the updated StatefulSet and if not, it
// return false.
func updatedStatefulSet(
	actual *appsv1.StatefulSet, cluster *myspec.M3DBCluster, isoGroup myspec.IsolationGroup,
) (*appsv1.StatefulSet, bool, error) {
	// The operator will only perform an update if the current StatefulSet has been
	// annotated to indicate that it is okay to update it.
	if val, ok := actual.Annotations[annotations.Update]; !ok || val != annotations.EnabledVal {
		str, ok := actual.Annotations[annotations.ParallelUpdate]
		if !ok {
			return nil, false, nil
		}

		if parallelVal, err := strconv.Atoi(str); err != nil {
			return nil, false, err
		} else if parallelVal < 1 {
			return nil, false, fmt.Errorf("parallel update value invalid: %v", str)
		}
	}

	expected, err := m3db.GenerateStatefulSet(cluster, isoGroup.Name, isoGroup.NumInstances)
	if err != nil {
		return nil, false, err
	}

	var update bool

	// The Kubernetes API server sets various default values for fields so instead of comparing
	// if the desired spec is equal to the actual spec, which may have fail because the desired
	// spec hasn't yet been transformed by the API server, we use DeepDerivative to only compare
	// those fields in the desired spec which are actually set. The controller has special logic
	// for handling changes to the number of replicas in the cluster since such changes also
	// require updates to the placement so we can safely ignore the replicas here.
	expected.Spec.Replicas = actual.Spec.Replicas
	if !equality.Semantic.DeepDerivative(expected.Spec, actual.Spec) {
		update = true
	}

	// We also want to check if the labels in the cluster spec have changed and update the
	// StatefulSet if so.
	if !reflect.DeepEqual(expected.Labels, actual.Labels) {
		update = true
	}

	// If we don't need to perform an update to the StatefulSet's spec, but the StatefulSet
	// has an update annotation, we'll still update the StatefulSet to remove the update
	// annotation. This ensures that users can always set an update annotation and then
	// wait for it to be removed to know if the operator has processed a StatefulSet.
	if !update {
		delete(actual.Annotations, annotations.Update)
		delete(actual.Annotations, annotations.ParallelUpdate)
		return actual, true, nil
	}

	// Ensure we keep old object meta so that resource version info can be used by
	// K8s API for conflict resolution.
	expected.ObjectMeta = *actual.ObjectMeta.DeepCopy()
	// Reset expected annotations since we ensure their final state below.
	expected.ObjectMeta.Annotations = map[string]string{}
	expected.Status = actual.DeepCopy().Status

	copyAnnotations(expected, actual)
	return expected, true, nil
}

func copyAnnotations(expected, actual *appsv1.StatefulSet) {
	// It's okay for users to add annotations to a StatefulSet after it has been created so
	// we'll want to copy over any that exist on the actual StatefulSet but not the expected
	// one. The only exception is we don't want to copy over the update annotation so users
	// will always have to explicitly trigger updates. Note that this means removing an
	// annotation will require users to delete the annotation from the StatefulSet as well as
	// the cluster spec.
	for k, v := range actual.Annotations {
		if k == annotations.Update {
			// NB(nate): add this check for backwards compatibility. Existing components
			// may be using the old update annotation. We want to ensure rollouts still work as
			// expected.
			if expected.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
				expected.Annotations[annotations.ParallelUpdateInProgress] = "1"
			}
			continue
		}

		// For parallel updates, remove the initial annotation added by the client and add rollout
		// in progress annotation. This will ensure that in future passes we don't attempt to
		// update the statefulset again unnecessarily and simply roll pods that need to pick up
		// updates.
		if k == annotations.ParallelUpdate {
			expected.Annotations[annotations.ParallelUpdateInProgress] = v
			continue
		}

		if _, ok := expected.Annotations[k]; !ok {
			expected.Annotations[k] = v
		}
	}
}
