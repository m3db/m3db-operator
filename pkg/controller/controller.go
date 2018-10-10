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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	samplescheme "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"
	clusterlisters "github.com/m3db/m3db-operator/pkg/client/listers/m3dboperator/v1"

	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	"github.com/uber-go/tally"
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	_controllerName = "m3db-controller"
	_workQueueName  = "M3DBClusters"
)

var (
	// ErrIsolationGroupsMissing indicates that the isolation groups within the
	// spec are missing
	ErrIsolationGroupsMissing = errors.New("no isolation groups  specified")

	// ErrInvalidReplicationFactor indicates that the replication factor within
	// the spec is missing
	ErrInvalidReplicationFactor = errors.New("invalid replication factor")
)

// Cluster contains the CRD for the M3 cluster
type Cluster struct {
	M3DBCluster *myspec.M3DBCluster
}

// Controller object
type Controller struct {
	lock            *sync.Mutex
	logger          *zap.Logger
	scope           tally.Scope
	k8sclient       k8sops.K8sops
	clusters        map[string]Cluster
	placementClient placement.Client
	namespaceClient namespace.Client
	doneCh          chan struct{}

	kubeClient kubernetes.Interface
	crdClient  clientset.Interface

	clusterLister      clusterlisters.M3DBClusterLister
	clustersSynced     cache.InformerSynced
	statefulSetLister  appslisters.StatefulSetLister
	statefulSetsSynced cache.InformerSynced
	podLister          corelisters.PodLister
	podsSynced         cache.InformerSynced

	workQueue workqueue.RateLimitingInterface
	recorder  eventer.Poster
}

// New creates new instance of Controller
func New(opts ...Option) (*Controller, error) {
	options := &options{}

	for _, o := range opts {
		o.execute(options)
	}

	if err := options.validate(); err != nil {
		return nil, err
	}

	kubeInformerFactory := options.kubeInformerFactory
	m3dbClusterInformerFactory := options.m3dbClusterInformerFactory
	kclient := options.kclient
	kubeClient := options.kubeClient
	crdClient := options.crdClient
	scope := options.scope

	logger := options.logger
	if logger == nil {
		logger = zap.NewNop()
	}

	var err error
	plClient := options.placementClient
	nsClient := options.namespaceClient
	if plClient == nil {
		// TODO(PS) Move these clients within the cluster object to ensure each
		// cluster has it's own configured client
		plClient, err = placement.NewClient(placement.WithLogger(logger))
		if err != nil {
			return nil, err
		}
	}

	if nsClient == nil {
		nsClient, err = namespace.NewClient(namespace.WithLogger(logger))
		if err != nil {
			return nil, err
		}
	}

	statefulSetInformer := kubeInformerFactory.Apps().V1().StatefulSets()
	// TODO(schallert): we may not need the pod informer here, but we want a
	// lister and not sure if getting lister from informer requires waiting for
	// pod informer cache.
	podInformer := kubeInformerFactory.Core().V1().Pods()
	m3dbClusterInformer := m3dbClusterInformerFactory.Operator().V1().M3DBClusters()

	samplescheme.AddToScheme(scheme.Scheme)

	workQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), _workQueueName)

	p := &Controller{
		lock:      &sync.Mutex{},
		logger:    logger,
		scope:     scope,
		k8sclient: kclient,
		clusters:  make(map[string]Cluster),
		doneCh:    make(chan struct{}),

		kubeClient: kubeClient,
		crdClient:  crdClient,

		clusterLister:      m3dbClusterInformer.Lister(),
		clustersSynced:     m3dbClusterInformer.Informer().HasSynced,
		statefulSetLister:  statefulSetInformer.Lister(),
		statefulSetsSynced: statefulSetInformer.Informer().HasSynced,
		podLister:          podInformer.Lister(),
		podsSynced:         podInformer.Informer().HasSynced,

		placementClient: plClient,
		namespaceClient: nsClient,

		workQueue: workQueue,
		// TODO(celina): figure out if we actually need a recorder for each namespace
		recorder: eventer.NewEventRecorder(kubeClient, eventer.WithLogger(logger), eventer.WithComponent(_controllerName)),
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

	return p, nil
}

// Init ensures all the required resources are created
func (c *Controller) Init() error {
	if err := c.k8sclient.CreateCRD(m3dboperator.Name); err != nil {
		return err
	}
	c.logger.Info("found existing", zap.Int("clusters", len(c.clusters)))
	return nil
}

// Run drives the controller event loop.
func (c *Controller) Run(nWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workQueue.ShutDown()

	c.logger.Info("starting Operator controller")

	c.logger.Info("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.clustersSynced, c.statefulSetsSynced, c.podsSynced); !ok {
		return errors.New("caches failed to sync")
	}

	c.logger.Info("starting workers")
	for i := 0; i < nWorkers; i++ {
		go wait.Until(c.runLoop, time.Second, stopCh)
	}

	c.logger.Info("workers started")
	<-stopCh
	c.logger.Info("shutting down workers")

	return nil
}

func (c *Controller) enqueueCluster(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workQueue.AddRateLimited(key)
	c.scope.Counter("enqueued_event").Inc(int64(1))
}

func (c *Controller) runLoop() {
	for c.processItem() {
	}
}

func (c *Controller) processItem() bool {
	obj, shutdown := c.workQueue.Get()
	c.scope.Counter("dequeued_event").Inc(int64(1))
	if shutdown {
		return false
	}

	// Closure so we can defer workQueue.Done.
	err := func(obj interface{}) error {
		defer c.workQueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string from queue, got %#v", obj))
			return nil
		}

		if err := c.handleClusterUpdate(key); err != nil {
			return fmt.Errorf("error syncing cluster '%s': %v", key, err)
		}

		c.workQueue.Forget(obj)
		c.logger.Info("successfully synced item", zap.String("key", key))

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}

func (c *Controller) handleClusterUpdate(key string) error {
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

	// MUST create a deep copy of the cluster or risk corruping cache! Technically
	// only need if we modify, but we frequently do that so let's deep copy to
	// start and remove unnecessary calls later to optimize if we want.
	cluster = cluster.DeepCopy()

	// TODO(schallert): propagate whether services were created back up to client
	// Per https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#statefulsetspec-v1-apps,
	// headless service MUST exist before statefulset.
	if err := c.ensureServices(cluster); err != nil {
		return err
	}

	isoGroups := cluster.Spec.IsolationGroups

	if len(isoGroups) == 0 {
		// nothing to do, no groups to create in
		return nil
	}

	zones := make([]string, len(isoGroups))
	for i := range zones {
		zones[i] = isoGroups[i].Name
	}

	childrenSets, err := c.getChildStatefulSets(cluster)
	if err != nil {
		return err
	}

	for _, sts := range childrenSets {
		// if any of the statefulsets aren't ready, wait until they are as we'll get
		// another event (ready == bootstrapped)
		if sts.Spec.Replicas != nil && *sts.Spec.Replicas != sts.Status.ReadyReplicas {
			// TODO(schallert): figure out what to do if replicas is not set
			c.logger.Info("waiting for statefulset to be ready", zap.String("name", sts.Name), zap.Int32("ready", sts.Status.ReadyReplicas))
			return nil
		}
	}

	// At this point all existing statefulsets are bootstrapped.

	if len(childrenSets) != len(zones) {
		nextID := len(childrenSets)
		// create a statefulset

		name := fmt.Sprintf("%s-%d", cluster.Name, nextID)
		sts, err := k8sops.GenerateStatefulSet(cluster, zones[nextID], isoGroups[nextID].NumInstances)
		if err != nil {
			return err
		}

		sts, err = c.kubeClient.AppsV1().StatefulSets(cluster.Namespace).Create(sts)
		if err != nil {
			c.logger.Error(err.Error())
			return err
		}

		c.logger.Info("created statefulset", zap.String("name", name))
		return nil
	}

	if !cluster.Status.HasInitializedNamespace() {
		updated, err := c.validateNamespaceWithStatus(cluster)
		if err != nil {
			c.recorder.WarningEvent(cluster, eventer.ReasonFailedToUpdate, "namespace not validated: %v", err.Error())
			return err
		}

		// TODO(schallert): Decide for sure what our conventions for re-enqueueing
		// will be.
		if updated {
			err = errors.New("re-enqueing cluster due to API update")
			c.recorder.WarningEvent(cluster, eventer.ReasonFailedToUpdate, err.Error())
		}
	}

	if !cluster.Status.HasInitializedPlacement() {
		updated, err := c.validatePlacementWithStatus(cluster)
		if err != nil {
			return err
		}

		if updated {
			err = errors.New("re-enqueing cluster due to API update")
			c.recorder.WarningEvent(cluster, eventer.ReasonFailedToUpdate, err.Error())
			return err
		}
	}

	c.logger.Info("nothing to do",
		zap.Int("childrensets", len(childrenSets)),
		zap.Int("zones", len(zones)),
		zap.Int64("generation", cluster.ObjectMeta.Generation),
		zap.String("rv", cluster.ObjectMeta.ResourceVersion))

	c.recorder.NormalEvent(cluster, eventer.ReasonSuccessfulUpdate, "Cluster update handled successfully")
	return nil
}

// This func is currently read-only, but if we end up modifying statefulsets
// we'll have to deepcopy.
func (c *Controller) getChildStatefulSets(cluster *myspec.M3DBCluster) ([]*appsv1.StatefulSet, error) {
	// TODO(schallert): mark sts w/ a label selector so we can query more
	// efficiently (already have label helpers in k8sops)
	statefulSets, err := c.statefulSetLister.StatefulSets(cluster.Namespace).List(labels.Everything())
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

func (c *Controller) handleStatefulSetUpdate(obj interface{}) {
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

	c.logger.Info("processing statefulset", zap.String("name", object.GetName()))

	owner := metav1.GetControllerOf(object)
	// TODO(schallert): const
	if owner == nil || owner.Kind != "m3dbcluster" {
		return
	}

	cluster, err := c.clusterLister.M3DBClusters(object.GetNamespace()).Get(owner.Name)
	if err != nil {
		c.logger.Info("ignoring orphaned object", zap.String("m3dbcluster", owner.Name), zap.String("statefulset", object.GetName()))
		return
	}

	// enqueue the cluster for processing
	c.enqueueCluster(cluster)
	c.recorder.NormalEvent(cluster, eventer.ReasonSuccessfulUpdate, "StatefulSet update handled successfully")
	return
}
