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

package podidentity

import (
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/tools/cache"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider(t *testing.T, opts ...Option) *provider {
	p, err := NewProvider(opts...)
	require.NoError(t, err)
	return p.(*provider)
}

func TestNewProvider(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	nodeLister := kubeInformer.Core().V1().Nodes().Lister()

	_, err := NewProvider()
	assert.Error(t, err)

	p, err := NewProvider(WithNodeLister(nodeLister))
	assert.NotNil(t, p)
	assert.NoError(t, err)
}

func TestIdentity(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []runtime.Object
		pod       *corev1.Pod
		cluster   *myspec.M3DBCluster
		expID     *myspec.PodIdentity
		expErrAny bool
		expErr    error
	}{
		{
			name: "bad config unknown source",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-0",
					UID:  types.UID("foo"),
				},
			},
			cluster:   clusterWithSources("foo", myspec.PodIdentitySource("badsource")),
			expErrAny: true,
		},
		{
			name: "bad config empty UID",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			cluster: &myspec.M3DBCluster{},
			expErr:  errEmptyPodSourceUID,
		},
		{
			name: "no config",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-0",
					UID:  types.UID("foo"),
				},
			},
			cluster: &myspec.M3DBCluster{},
			expID: &myspec.PodIdentity{
				Name: "pod-0",
				UID:  "foo",
			},
		},
		{
			name: "UID config",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					UID:  types.UID("foo"),
				},
			},
			cluster: clusterWithSources("foo", myspec.PodIdentitySourcePodUID),
			expID: &myspec.PodIdentity{
				UID:  "foo",
				Name: "pod1",
			},
		},
		{
			name: "node provider ID config",
			pod:  newPodForNode("pod-b", "node-2"),
			nodes: []runtime.Object{func() runtime.Object {
				n := newTestNode("node-2")
				n.Spec.ProviderID = "id2"
				return n
			}()},
			cluster: clusterWithSources("foo", myspec.PodIdentitySourceNodeSpecProviderID),
			expID: &myspec.PodIdentity{
				Name:           "pod-b",
				NodeProviderID: "id2",
			},
		},
		{
			name: "node name config",
			pod:  newPodForNode("pod-b", "node-2"),
			nodes: []runtime.Object{func() runtime.Object {
				n := newTestNode("node-2")
				return n
			}()},
			cluster: clusterWithSources("foo", myspec.PodIdentitySourceNodeName),
			expID: &myspec.PodIdentity{
				Name:     "pod-b",
				NodeName: "node-2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := kubefake.NewSimpleClientset(test.nodes...)
			kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			nodeLister := kubeInformer.Core().V1().Nodes().Lister()
			stopCh := make(chan struct{})
			go kubeInformer.Start(stopCh)
			defer func() {
				close(stopCh)
			}()

			err := wait.Poll(time.Millisecond, 5*time.Second, func() (bool, error) {
				p := newTestProvider(t, WithNodeLister(nodeLister))
				id, err := p.Identity(test.pod, test.cluster)
				if test.expErrAny {
					assert.Error(t, err)
					return true, nil
				}
				if test.expErr != nil {
					assert.Equal(t, test.expErr, err)
					return true, nil
				}

				if err != nil {
					return false, nil
				}
				assert.NoError(t, err)
				assert.Equal(t, test.expID, id)
				return true, nil
			})
			assert.NoError(t, err)
		})
	}
}

func TestIdentityJSON(t *testing.T) {
	id := &myspec.PodIdentity{
		Name: "foo",
		UID:  "bar",
	}

	exp := `{"name":"foo","uid":"bar"}`
	res, err := IdentityJSON(id)
	assert.NoError(t, err)
	assert.Equal(t, exp, res)
}

func TestNodeForPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-a",
		},
	}

	kubeClient := kubefake.NewSimpleClientset()
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	nodeLister := kubeInformer.Core().V1().Nodes().Lister()

	stopCh := make(chan struct{})
	go kubeInformer.Start(stopCh)
	defer func() {
		close(stopCh)
	}()
	cache.WaitForCacheSync(stopCh, kubeInformer.Core().V1().Nodes().Informer().HasSynced)

	p := newTestProvider(t, WithNodeLister(nodeLister))

	_, err := p.nodeForPod(pod)
	assert.True(t, strings.Contains(err.Error(), "not yet scheduled"),
		"expected not yet scheduled error")

	pod.Spec.NodeName = "node-1"
	_, err = p.nodeForPod(pod)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err), "expected error to be notfound error")

	testNode := newTestNode("node-1")

	_, err = kubeClient.CoreV1().Nodes().Create(context.Background(), testNode, metav1.CreateOptions{})
	require.NoError(t, err)

	// Might need to wait for informer to sync.
	err = wait.Poll(time.Millisecond, 5*time.Second, func() (bool, error) {
		node, err := p.nodeForPod(pod)
		if err != nil {
			return false, nil
		}
		assert.NoError(t, err)
		assert.Equal(t, testNode, node)
		return true, nil
	})

	assert.NoError(t, err)
}

func clusterWithSources(clusterName string, sources ...myspec.PodIdentitySource) *myspec.M3DBCluster {
	return &myspec.M3DBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec: myspec.ClusterSpec{
			PodIdentityConfig: &myspec.PodIdentityConfig{
				Sources: sources,
			},
		},
	}
}

func newPodForNode(podName, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
}

func newTestNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
