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
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"
	"github.com/m3db/m3db-operator/pkg/m3admin/topic"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	"github.com/m3db/m3/src/msg/generated/proto/topicpb"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestAdminClient(cl m3admin.Client, url string) *multiAdminClient {
	m := newMultiAdminClient(nil, zap.NewNop())
	m.clusterKeyFn = func(cl *myspec.M3DBCluster, url string) string {
		return cl.GetObjectMeta().GetName()
	}
	m.clusterURLFn = func(*myspec.M3DBCluster) string {
		return url
	}
	m.adminClientFn = func(...m3admin.Option) m3admin.Client {
		return cl
	}
	return m
}

func TestClusterKey(t *testing.T) {
	cluster := newM3DBCluster("ns", "a")
	key := clusterKey(cluster, "clustera.local")
	assert.Equal(t, "ns/a/clustera.local", key)
}

func TestClusterURL(t *testing.T) {
	cluster := newM3DBCluster("ns", "a")
	cluster.Namespace = "foo"
	url := clusterURL(cluster)
	assert.Equal(t, "http://m3coordinator-a.foo:7201", url)

	cluster.Spec.ExternalCoordinator = &myspec.ExternalCoordinatorConfig{
		ServiceEndpoint: "my-custom-coordinator.other-namespace:1234",
	}
	url = clusterURL(cluster)
	assert.Equal(t, "http://my-custom-coordinator.other-namespace:1234", url)
}

func TestClusterURLProxy(t *testing.T) {
	cluster := newM3DBCluster("ns", "a")
	cluster.Namespace = "foo"

	url := clusterURLProxy(cluster)
	assert.Equal(t, "http://localhost:8001/api/v1/namespaces/foo/services/m3coordinator-a:coordinator/proxy", url)
}

func TestNewMultiAdminClient(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	m := newMultiAdminClient(nil, zap.NewNop())
	assert.NotNil(t, m)

	// Ensure no necessary fields are nil.
	for _, v := range []interface{}{
		m.plClients,
		m.plClients,
		m.plClientFn,
		m.plClientFn,
		m.clusterKeyFn,
		m.clusterURLFn,
		m.adminClientFn,
		m.logger,
	} {
		assert.NotNil(t, v)
	}
}

func newM3DBCluster(ns, name string) *myspec.M3DBCluster {
	return &myspec.M3DBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}

func TestNamespaceClientForCluster(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	m3Client := m3admin.NewMockClient(mc)
	nsClient := namespace.NewMockClient(mc)

	m := newTestAdminClient(m3Client, "http://foo")
	m.nsClientFn = func(_ ...namespace.Option) (namespace.Client, error) {
		return nsClient, nil
	}

	clusterA := newM3DBCluster("ns", "a")
	clusterB := newM3DBCluster("ns", "b")
	clusterC := newM3DBCluster("ns", "c")
	testErr := errors.New("test")

	cl := m.namespaceClientForCluster(clusterA)
	assert.Equal(t, nsClient, cl)
	_ = m.namespaceClientForCluster(clusterB)
	assert.Equal(t, 2, len(m.nsClients))
	m.nsClientFn = func(_ ...namespace.Option) (namespace.Client, error) {
		return nil, testErr
	}

	cl2 := m.namespaceClientForCluster(clusterA)
	assert.Equal(t, cl, cl2)

	cl3 := m.namespaceClientForCluster(clusterC)
	assert.Equal(t, testErr, cl3.Delete("foo"))
}

func TestPlacementClientForCluster(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	m3Client := m3admin.NewMockClient(mc)
	plClient := placement.NewMockClient(mc)

	m := newTestAdminClient(m3Client, "http://foo")
	m.plClientFn = func(_ ...placement.Option) (placement.Client, error) {
		return plClient, nil
	}

	clusterA := newM3DBCluster("ns", "a")
	clusterB := newM3DBCluster("ns", "b")
	clusterC := newM3DBCluster("ns", "c")
	testErr := errors.New("test")

	cl := m.placementClientForCluster(clusterA)
	assert.Equal(t, plClient, cl)
	_ = m.placementClientForCluster(clusterB)
	assert.Equal(t, 2, len(m.plClients))
	m.plClientFn = func(_ ...placement.Option) (placement.Client, error) {
		return nil, testErr
	}

	cl2 := m.placementClientForCluster(clusterA)
	assert.Equal(t, cl, cl2)

	cl3 := m.placementClientForCluster(clusterC)
	assert.Equal(t, testErr, cl3.Delete())
}

func TestTopicClientForCluster(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	m3Client := m3admin.NewMockClient(mc)
	tpClient := topic.NewMockClient(mc)

	m := newTestAdminClient(m3Client, "http://foo")
	m.tpClientFn = func(_ ...topic.Option) (topic.Client, error) {
		return tpClient, nil
	}

	clusterA := newM3DBCluster("ns", "a")
	clusterB := newM3DBCluster("ns", "b")
	clusterC := newM3DBCluster("ns", "c")
	testErr := errors.New("test")

	cl := m.topicClientForCluster(clusterA)
	assert.Equal(t, tpClient, cl)
	_ = m.topicClientForCluster(clusterB)
	assert.Equal(t, 2, len(m.tpClients))
	m.tpClientFn = func(_ ...topic.Option) (topic.Client, error) {
		return nil, testErr
	}

	cl2 := m.topicClientForCluster(clusterA)
	assert.Equal(t, cl, cl2)

	cl3 := m.topicClientForCluster(clusterC)
	assert.Equal(t, testErr, cl3.Delete("topic"))
}

func TestErrorNamespaceClient(t *testing.T) {
	clErr := errors.New("test")
	cl := newErrorNamespaceClient(clErr)

	err := cl.Create(nil)
	assert.Equal(t, clErr, err)

	r, err := cl.List()
	assert.Nil(t, r)
	assert.Equal(t, clErr, err)

	err = cl.Delete("foo")
	assert.Equal(t, clErr, err)
}

func TestErrorPlacementClient(t *testing.T) {
	clErr := errors.New("test")
	cl := newErrorPlacementClient(clErr)

	err := cl.Init(nil)
	assert.Equal(t, clErr, err)

	pl, err := cl.Get()
	assert.Nil(t, pl)
	assert.Equal(t, clErr, err)

	err = cl.Delete()
	assert.Equal(t, clErr, err)

	err = cl.Add([]*placementpb.Instance{&placementpb.Instance{}})
	assert.Equal(t, clErr, err)

	err = cl.Remove("foo")
	assert.Equal(t, clErr, err)

	err = cl.Replace("foo", placementpb.Instance{})
	assert.Equal(t, clErr, err)
}

func TestErrorTopicClient(t *testing.T) {
	clErr := errors.New("test")
	cl := newErrorTopicClient(clErr)

	err := cl.Init("topic", nil)
	assert.Equal(t, clErr, err)

	tp, err := cl.Get("topic")
	assert.Nil(t, tp)
	assert.Equal(t, clErr, err)

	err = cl.Delete("topic")
	assert.Equal(t, clErr, err)

	err = cl.Add("topic", &topicpb.ConsumerService{})
	assert.Equal(t, clErr, err)

	err = cl.Delete("topic")
	assert.Equal(t, clErr, err)
}
