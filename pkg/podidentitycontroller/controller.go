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

package podidentitycontroller

import (
	"errors"
	"fmt"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	samplescheme "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"
	clusterlisters "github.com/m3db/m3db-operator/pkg/client/listers/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

const (
	controllerName = "m3db-podidentity-controller"
	workQueueName  = "m3db-pod-identity"
)

var (
	errOrphanedPod = errors.New("pod does not belong to an m3db cluster")
)

// Controller object.
type Controller struct {
	logger     *zap.Logger
	clock      clock.Clock
	scope      tally.Scope
	k8sclient  k8sops.K8sops
	doneCh     chan struct{}
	idProvider podidentity.Provider
	// adminClient

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

// New creates a new pod identity controller.
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

	statefulSetInformer := kubeInformerFactory.Apps().V1().StatefulSets()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	m3dbClusterInformer := m3dbClusterInformerFactory.Operator().V1().M3DBClusters()

	samplescheme.AddToScheme(scheme.Scheme)

	workQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName)

	r, err := eventer.NewEventRecorder(eventer.WithClient(kubeClient), eventer.WithLogger(logger), eventer.WithComponent(controllerName))
	if err != nil {
		return nil, err
	}

	c := &Controller{
		logger:    logger,
		scope:     scope,
		clock:     clock.RealClock{},
		k8sclient: kclient,
		// adminClient: multiClient,
		doneCh:     make(chan struct{}),
		idProvider: options.idProvider,

		kubeClient: kubeClient,
		crdClient:  crdClient,

		clusterLister:      m3dbClusterInformer.Lister(),
		clustersSynced:     m3dbClusterInformer.Informer().HasSynced,
		statefulSetLister:  statefulSetInformer.Lister(),
		statefulSetsSynced: statefulSetInformer.Informer().HasSynced,
		podLister:          podInformer.Lister(),
		podsSynced:         podInformer.Informer().HasSynced,

		workQueue: workQueue,
		recorder:  r,
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			c.enqueuePod(new)
		},
		DeleteFunc: func(obj interface{}) {
			// No-op
		},
	})

	return c, nil
}

func (c *Controller) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		c.logger.Error("error splitting pod cache key", zap.Error(err))
		return
	}
	c.workQueue.AddRateLimited(key)
	c.scope.Counter("enqueued_event").Inc(int64(1))
}

// Run starts the controller event loop.
func (c *Controller) Run(nWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workQueue.ShutDown()

	c.logger.Info("starting controller")
	c.logger.Info("waiting for informer caches sync")
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

		if err := c.handlePodEvent(key); err != nil {
			return fmt.Errorf("error syncing cluster '%s': %v", key, err)
		}

		c.workQueue.Forget(obj)
		c.logger.Debug("successfully synced item", zap.String("key", key))

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}

func (c *Controller) handlePodEvent(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.Error("invalid resource key", zap.Error(err))
		return nil
	}

	pod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			c.logger.Error("pod no longer exists", zap.String("pod", name))
			return nil
		}

		return err
	}

	if pod == nil {
		return errors.New("got nil pod for key " + key)
	}

	return c.handlePodUpdate(pod)
}

func (c *Controller) handlePodUpdate(pod *corev1.Pod) error {
	// We only process pods that are members of m3db clusters.
	if _, found := getClusterValue(pod); !found {
		return nil
	}

	pod = pod.DeepCopy()

	podLogger := c.logger.With(zap.String("pod", pod.Name))

	podLogger.Info("processing pod")

	cluster, err := c.getParentCluster(pod)
	if err != nil {
		podLogger.Error("error getting parent cluster", zap.Error(err))
		return err
	}

	id, err := c.idProvider.Identity(pod, cluster)
	if err != nil {
		podLogger.Error("error getting pod ID", zap.Error(err))
		return err
	}

	idStr, err := podidentity.IdentityJSON(id)
	if err != nil {
		podLogger.Error("error marshaling pod ID", zap.Error(err))
		return err
	}

	podLogger.Info("pod ID", zap.Any("id", id))

	currentID, ok := pod.Annotations[podidentity.AnnotationKeyPodIdentity]
	if ok {
		if currentID != idStr {
			podLogger.Warn("pod ID mismatch",
				zap.String("currentID", currentID),
				zap.String("newID", idStr))
		}

		// TODO(schallert): decide how to enforce updated pod identity (need to
		// determine ramnifications of changing).
		return nil
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[podidentity.AnnotationKeyPodIdentity] = idStr
	_, err = c.kubeClient.CoreV1().Pods(pod.Namespace).Update(pod)
	if err != nil {
		podLogger.Error("error updating pod annotation", zap.Error(err))
		return err
	}

	return nil
}

func getClusterValue(pod *corev1.Pod) (string, bool) {
	cluster, ok := pod.Labels[labels.Cluster]
	if !ok {
		return "", false
	}

	return cluster, true
}

func (c *Controller) getParentCluster(pod *corev1.Pod) (*myspec.M3DBCluster, error) {
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
