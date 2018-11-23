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
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	clientsetfake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	m3dbinformers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	kubeClient := kubefake.NewSimpleClientset()
	crdClient := clientsetfake.NewSimpleClientset()
	client := k8sops.NewMockK8sops(mc)
	idProvider := podidentity.NewMockProvider(mc)
	testOpts := []Option{
		WithScope(tally.NoopScope),
		WithLogger(zap.NewNop()),
		WithKClient(client),
		WithCRDClient(crdClient),
		WithKubeClient(kubeClient),
		WithPodIdentityProvider(idProvider),
		WithKubeInformerFactory(kubeinformers.NewSharedInformerFactory(kubeClient, 0)),
		WithM3DBClusterInformerFactory(m3dbinformers.NewSharedInformerFactory(crdClient, 0)),
	}

	controller, err := New(testOpts...)
	assert.NoError(t, err)
	assert.NotNil(t, controller)
}

func TestGetClusterValue(t *testing.T) {
	pod := &corev1.Pod{}
	cluster, ok := getClusterValue(pod)
	assert.False(t, ok)
	assert.Equal(t, "", cluster)

	pod.Labels = map[string]string{
		"operator.m3db.io/cluster": "foo",
	}

	cluster, ok = getClusterValue(pod)
	assert.True(t, ok)
	assert.Equal(t, "foo", cluster)
}

func TestGetParentCluster(t *testing.T) {
	cluster := &myspec.M3DBCluster{
		ObjectMeta: newObjectMeta("foo", nil),
	}

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{
			cluster,
		},
	})
	defer deps.cleanup()

	c := deps.newController()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
		},
	}
	_, err := c.getParentCluster(pod)
	assert.Equal(t, errOrphanedPod, err)

	pod.Labels = map[string]string{
		"operator.m3db.io/cluster": "foo",
	}

	parentCluster, err := c.getParentCluster(pod)
	assert.NoError(t, err)
	assert.Equal(t, cluster, parentCluster)
}

func TestHandlePodUpdate(t *testing.T) {
	cluster := &myspec.M3DBCluster{
		ObjectMeta: newObjectMeta("foo", nil),
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "namespace",
			Labels: map[string]string{
				"operator.m3db.io/cluster": "foo",
			},
		},
	}

	deps := newTestDeps(t, &testOpts{
		crdObjects:  []runtime.Object{cluster},
		kubeObjects: []runtime.Object{pod1},
	})
	defer deps.cleanup()

	c := deps.newController()

	mockID := &myspec.PodIdentity{
		Name: "pod1",
		UID:  "foo",
	}

	deps.idProvider.EXPECT().Identity(pod1, cluster).Return(mockID, nil)

	err := c.handlePodUpdate(pod1)
	require.NoError(t, err)

	newPod, err := deps.kubeClient.CoreV1().Pods("namespace").Get("pod1", metav1.GetOptions{})
	assert.NoError(t, err)

	annotatedID, ok := newPod.Annotations["operator.m3db.io/pod-identity"]
	require.True(t, ok, "new pod must have annotated ID")
	expID := `{"name":"pod1","uid":"foo"}`
	assert.Equal(t, expID, annotatedID)
}
