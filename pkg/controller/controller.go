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
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	samplescheme "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"
	clusterlisters "github.com/m3db/m3db-operator/pkg/client/listers/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	m3placement "github.com/m3db/m3/src/cluster/placement"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	jsonpatch "github.com/evanphx/json-patch"
	pkgerrors "github.com/pkg/errors"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
	"k8s.io/utils/pointer"
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

// Configuration contains parameters for the controller.
type Configuration struct {
	// ManageCRD indicates whether the controller should create and update specs
	// of the CRDs it controls.
	ManageCRD bool

	// EnableValidation controls whether OpenAPI validation is enabled on the CRD.
	EnableValidation bool
}

type controllerBase struct {
	lock   *sync.Mutex
	logger *zap.Logger
	clock  clock.Clock
	scope  tally.Scope
	config Configuration
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
			config: options.config,
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
	if c.config.ManageCRD {
		if err := c.k8sclient.CreateOrUpdateCRD(m3dboperator.M3DBClustersName, c.config.EnableValidation); err != nil {
			return pkgerrors.WithMessage(err, "could not create or update CRD")
		}
	}

	c.logger.Info("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.clustersSynced, c.statefulSetsSynced, c.podsSynced); !ok {
		return errors.New("caches failed to sync")
	}

	c.logger.Info("starting workers")
	for i := 0; i < nWorkers; i++ {
		go c.runClusterLoop()
		go c.runPodLoop()
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

func (c *M3DBController) runClusterLoop() {
	for c.processClusterQueueItem() {
	}
}

func (c *M3DBController) processClusterQueueItem() bool {
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

		if err := c.handleClusterEvent(key); err != nil {
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

func (c *M3DBController) handleClusterEvent(key string) error {
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

	return c.handleClusterUpdate(cluster)
}

// We are guaranteed by handleClusterEvent that we will never be passed a nil
// cluster here.
func (c *M3DBController) handleClusterUpdate(cluster *myspec.M3DBCluster) error {
	// MUST create a deep copy of the cluster or risk corrupting cache! Technically
	// only need if we modify, but we frequently do that so let's deep copy to
	// start and remove unnecessary calls later to optimize if we want.
	cluster = cluster.DeepCopy()

	clusterLogger := c.logger.With(zap.String("cluster", cluster.Name))

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

		if _, err := c.removeEtcdFinalizer(cluster); err != nil {
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
		cluster, err = c.ensureEtcdFinalizer(cluster)
		if err != nil {
			return err
		}
	}

	if err := c.ensureConfigMap(cluster); err != nil {
		clusterLogger.Error("failed to ensure configmap", zap.Error(err))
		c.recorder.WarningEvent(cluster, eventer.ReasonFailSync, "failed to ensure configmap: %s", err.Error())
		return err
	}

	// Per https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#statefulsetspec-v1-apps,
	// headless service MUST exist before statefulset.
	if err := c.ensureServices(cluster); err != nil {
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

			_, err = c.kubeClient.AppsV1().StatefulSets(cluster.Namespace).Create(sts)
			if err != nil {
				c.logger.Error(err.Error())
				return err
			}

			c.logger.Info("created statefulset", zap.String("name", name))
			return nil
		}
	}

	// If any of the statefulsets aren't ready, wait until they are as we'll get
	// another event (ready == bootstrapped)
	for _, sts := range childrenSets {
		if sts.Spec.Replicas != nil && *sts.Spec.Replicas != sts.Status.ReadyReplicas {
			// TODO(schallert): figure out what to do if replicas is not set
			c.logger.Info("waiting for statefulset to be ready", zap.String("name", sts.Name), zap.Int32("ready", sts.Status.ReadyReplicas))
			return nil
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

	if !cluster.Status.HasInitializedPlacement() {
		cluster, err = c.validatePlacementWithStatus(cluster)
		if err != nil {
			return err
		}
	}

	// At this point we have the desired number of statefulsets, and every pod
	// across those sets is bootstrapped. However some may be bootstrapped because
	// they own no shards. Check to see that all pods are in the placement.
	selector := klabels.SelectorFromSet(labels.BaseLabels(cluster))
	pods, err := c.podLister.Pods(cluster.Namespace).List(selector)
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
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
		err = c.replacePodInPlacement(cluster, placement, leavingInstanceID, podToReplace)
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
				return c.expandPlacementForSet(cluster, set, group, placement)
			}
		}

		// If there are more pods in the placement than we want in the group,
		// trigger a remove so that we can shrink the set.
		if inPlacement > desired {
			setLogger.Info("remove instance from placement for set")
			return c.shrinkPlacementForSet(cluster, set, placement)
		}

		var newCount int32
		if current < desired {
			newCount = current + 1
		} else {
			newCount = current - 1
		}
		setLogger.Info("resizing set, desired != current", zap.Int32("newSize", newCount))

		setBytes, err := json.Marshal(set)
		if err != nil {
			return err
		}

		set.Spec.Replicas = pointer.Int32Ptr(newCount)

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
			Patch(set.Name, types.MergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("error updating statefulset %s: %v", set.Name, err)
		}

		return nil
	}

	placement, err = c.adminClient.placementClientForCluster(cluster).Get()
	if err != nil {
		return fmt.Errorf("error fetching placement: %v", err)
	}

	// TODO(celina): possibly do a replacement check here

	// See if we need to clean up the pod bootstrapping status.
	cluster, err = c.reconcileBootstrappingStatus(cluster, placement)
	if err != nil {
		return fmt.Errorf("error reconciling bootstrap status: %v", err)
	}

	c.logger.Info("nothing to do",
		zap.Int("childrensets", len(childrenSets)),
		zap.Int("zones", len(isoGroups)),
		zap.Int64("generation", cluster.ObjectMeta.Generation),
		zap.String("rv", cluster.ObjectMeta.ResourceVersion))

	return nil
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

func (c *M3DBController) runPodLoop() {
	for c.processPodQueueItem() {
	}
}

func (c *M3DBController) processPodQueueItem() bool {
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

		if err := c.handlePodEvent(key); err != nil {
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

func (c *M3DBController) handlePodEvent(key string) error {
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

	return c.handlePodUpdate(pod)
}

func (c *M3DBController) handlePodUpdate(pod *corev1.Pod) error {
	// We only process pods that are members of m3db clusters.
	if _, found := getClusterValue(pod); !found {
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
		Patch(pod.Name, types.MergePatchType, patchBytes)
	if err != nil {
		podLogger.Error("error updating pod annotation", zap.Error(err))
		return err
	}

	podLogger.Info("updated pod ID", zap.Any("id", id))
	c.recorder.NormalEvent(pod, eventer.ReasonSuccessSync, "updated pod %s with ID annotation", pod.Name)

	return nil
}

func getClusterValue(pod *corev1.Pod) (string, bool) {
	cluster, ok := pod.Labels[labels.Cluster]
	if !ok {
		return "", false
	}

	return cluster, true
}

func (c *M3DBController) getParentCluster(pod *corev1.Pod) (*myspec.M3DBCluster, error) {
	clusterName, found := getClusterValue(pod)
	if !found {
		return nil, errOrphanedPod
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
