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
	"sync/atomic"
	"testing"
	"time"

	crdfake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	crdinformers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	crdlisters "github.com/m3db/m3db-operator/pkg/client/listers/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

type testOpts struct {
	kubeObjects []runtime.Object
	crdObjects  []runtime.Object
	clock       clock.Clock
}

type testDeps struct {
	kubeClient        *kubefake.Clientset
	crdClient         *crdfake.Clientset
	idProvider        *podidentity.MockProvider
	statefulSetLister appsv1listers.StatefulSetLister
	podLister         corev1listers.PodLister
	crdLister         crdlisters.M3DBClusterLister
	placementClient   *placement.MockClient
	namespaceClient   *namespace.MockClient
	clock             clock.Clock
	mockController    *gomock.Controller
	stopCh            chan struct{}
	closed            int32
}

func (deps *testDeps) newController(t *testing.T) *M3DBController {
	logger := zap.NewNop()
	m := newMultiAdminClient(nil, zap.NewNop())
	m.nsClientFn = func(...namespace.Option) (namespace.Client, error) {
		return deps.namespaceClient, nil
	}
	m.plClientFn = func(...placement.Option) (placement.Client, error) {
		return deps.placementClient, nil
	}
	k8sopsClient, err := m3db.New(
		m3db.WithKClient(deps.kubeClient),
		m3db.WithCRDClient(deps.crdClient),
		m3db.WithLogger(logger))

	require.NoError(t, err)

	return &M3DBController{
		controllerBase: controllerBase{
			logger:      logger,
			scope:       tally.NoopScope,
			clock:       deps.clock,
			adminClient: m,

			kubeClient: deps.kubeClient,
			crdClient:  deps.crdClient,

			podWorkQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), podWorkQueueName),
			podLister:    deps.podLister,

			statefulSetLister:      deps.statefulSetLister,
			statefulSetCheckpoints: make(map[string]int64),
			recorder:               eventer.NewNopPoster(),
		},

		k8sclient:     k8sopsClient,
		podIDProvider: deps.idProvider,

		clusterWorkQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), clusterWorkQueueName),
		clusterLister:    deps.crdLister,
	}
}

func (deps *testDeps) cleanup() {
	if atomic.CompareAndSwapInt32(&deps.closed, 0, 1) {
		close(deps.stopCh)
		deps.mockController.Finish()
	}
}

func newTestDeps(t *testing.T, opts *testOpts) *testDeps {
	deps := &testDeps{
		kubeClient:     kubefake.NewSimpleClientset(opts.kubeObjects...),
		crdClient:      crdfake.NewSimpleClientset(opts.crdObjects...),
		clock:          opts.clock,
		stopCh:         make(chan struct{}),
		mockController: gomock.NewController(t),
	}

	deps.placementClient = placement.NewMockClient(deps.mockController)
	deps.namespaceClient = namespace.NewMockClient(deps.mockController)
	deps.idProvider = podidentity.NewMockProvider(deps.mockController)

	if deps.clock == nil {
		deps.clock = clock.NewFakeClock(time.Now())
	}

	kubeInformers := kubeinformers.NewSharedInformerFactory(deps.kubeClient, 0)
	sets := kubeInformers.Apps().V1().StatefulSets()
	pods := kubeInformers.Core().V1().Pods()

	crdInformers := crdinformers.NewSharedInformerFactory(deps.crdClient, 0)
	crds := crdInformers.Operator().V1alpha1().M3DBClusters()

	deps.statefulSetLister = sets.Lister()
	deps.podLister = pods.Lister()
	deps.crdLister = crds.Lister()

	go kubeInformers.Start(deps.stopCh)
	go crdInformers.Start(deps.stopCh)

	warmCh := make(chan bool)
	go func() {
		warmCh <- cache.WaitForCacheSync(deps.stopCh,
			sets.Informer().HasSynced,
			pods.Informer().HasSynced,
			crds.Informer().HasSynced,
		)
	}()

	select {
	case success := <-warmCh:
		require.True(t, success)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for informer cache")
	}

	return deps
}

func newObjectMeta(name string, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Labels:    labels,
		Namespace: "namespace",
	}
}
