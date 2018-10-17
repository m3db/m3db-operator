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
	crdlisters "github.com/m3db/m3db-operator/pkg/client/listers/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

type testOpts struct {
	kubeObjects     []runtime.Object
	crdObjects      []runtime.Object
	clock           clock.Clock
	placementClient placement.Client
	namespaceClient namespace.Client
}

type testDeps struct {
	kubeClient        *kubefake.Clientset
	crdClient         *crdfake.Clientset
	statefulSetLister appsv1listers.StatefulSetLister
	podLister         corev1listers.PodLister
	crdLister         crdlisters.M3DBClusterLister
	placementClient   placement.Client
	namespaceClient   namespace.Client
	clock             clock.Clock
	stopCh            chan struct{}
	closed            int32
}

func (deps *testDeps) newController() *Controller {
	return &Controller{
		logger: zap.NewNop(),
		scope:  tally.NoopScope,
		clock:  deps.clock,

		kubeClient: deps.kubeClient,
		crdClient:  deps.crdClient,

		clusterLister:     deps.crdLister,
		statefulSetLister: deps.statefulSetLister,
		podLister:         deps.podLister,

		placementClient: deps.placementClient,
		namespaceClient: deps.namespaceClient,
	}
}

func (deps *testDeps) cleanup() {
	if atomic.CompareAndSwapInt32(&deps.closed, 0, 1) {
		close(deps.stopCh)
	}
}

func newTestDeps(t *testing.T, opts *testOpts) *testDeps {
	deps := &testDeps{
		kubeClient:      kubefake.NewSimpleClientset(opts.kubeObjects...),
		crdClient:       crdfake.NewSimpleClientset(opts.crdObjects...),
		clock:           opts.clock,
		stopCh:          make(chan struct{}),
		placementClient: opts.placementClient,
		namespaceClient: opts.namespaceClient,
	}

	if deps.clock == nil {
		deps.clock = clock.NewFakeClock(time.Now())
	}

	kubeInformers := kubeinformers.NewSharedInformerFactory(deps.kubeClient, 0)
	sets := kubeInformers.Apps().V1().StatefulSets()
	pods := kubeInformers.Core().V1().Pods()

	crdInformers := crdinformers.NewSharedInformerFactory(deps.crdClient, 0)
	crds := crdInformers.Operator().V1().M3DBClusters()

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
