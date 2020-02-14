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
	"fmt"
	"sync"

	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"
	"github.com/m3db/m3db-operator/pkg/m3admin/topic"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	m3placement "github.com/m3db/m3/src/cluster/placement"
	"github.com/m3db/m3/src/msg/generated/proto/topicpb"
	m3topic "github.com/m3db/m3/src/msg/topic"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go.uber.org/zap"
)

// multiAdminClient wraps multiple m3admin placement and namespace clients based
// on the cluster they're pointed at.
type multiAdminClient struct {
	mu        sync.RWMutex
	nsClients map[string]namespace.Client
	plClients map[string]placement.Client
	tpClients map[string]topic.Client

	nsClientFn func(...namespace.Option) (namespace.Client, error)
	plClientFn func(...placement.Option) (placement.Client, error)
	tpClientFn func(...topic.Option) (topic.Client, error)

	clusterKeyFn func(metav1.ObjectMetaAccessor, string) string
	clusterURLFn func(metav1.ObjectMetaAccessor) string

	adminClientFn func(...m3admin.Option) m3admin.Client
	adminOpts     []m3admin.Option
	logger        *zap.Logger
}

// clusterKey returns a map key for a given cluster.
func clusterKey(obj metav1.ObjectMetaAccessor, url string) string {
	m := obj.GetObjectMeta()
	return m.GetNamespace() + "/" + m.GetName() + "/" + url
}

// clusterURL returns the URL to hit
func clusterURL(cluster metav1.ObjectMetaAccessor) string {
	m := cluster.GetObjectMeta()
	serviceName := m3db.CoordinatorServiceName(m.GetName())
	urlFmt := "http://%s.%s:%d"
	url := fmt.Sprintf(urlFmt, serviceName, m.GetNamespace(), m3db.PortM3Coordinator)
	return url
}

func newAdminClient(opts ...m3admin.Option) m3admin.Client {
	return m3admin.NewClient(opts...)
}

// clusterURLProxy returns a URL for communicating with a cluster via an
// intermediary kubectl proxy.
func clusterURLProxy(cluster metav1.ObjectMetaAccessor) string {
	m := cluster.GetObjectMeta()
	serviceName := m3db.CoordinatorServiceName(m.GetName())
	urlFmt := "http://localhost:8001/api/v1/namespaces/%s/services/%s:coordinator/proxy"
	url := fmt.Sprintf(urlFmt, m.GetNamespace(), serviceName)
	return url
}

func newMultiAdminClient(adminOpts []m3admin.Option, logger *zap.Logger) *multiAdminClient {
	return &multiAdminClient{
		nsClients:     make(map[string]namespace.Client),
		plClients:     make(map[string]placement.Client),
		tpClients:     make(map[string]topic.Client),
		nsClientFn:    namespace.NewClient,
		plClientFn:    placement.NewClient,
		tpClientFn:    topic.NewClient,
		clusterKeyFn:  clusterKey,
		clusterURLFn:  clusterURL,
		adminClientFn: newAdminClient,
		adminOpts:     adminOpts,
		logger:        logger,
	}
}

func (m *multiAdminClient) adminClientForCluster(cluster metav1.ObjectMetaAccessor) m3admin.Client {
	env := k8sops.DefaultM3ClusterEnvironmentName(cluster)
	opts := append(m.adminOpts, m3admin.WithEnvironment(env))
	return m.adminClientFn(opts...)
}

func (m *multiAdminClient) namespaceClientForCluster(cluster metav1.ObjectMetaAccessor) namespace.Client {
	url := m.clusterURLFn(cluster)
	key := m.clusterKeyFn(cluster, url)

	m.mu.RLock()
	client, ok := m.nsClients[key]
	m.mu.RUnlock()
	if ok {
		return client
	}

	adminClient := m.adminClientForCluster(cluster)
	client, err := m.nsClientFn(
		namespace.WithClient(adminClient),
		namespace.WithLogger(m.logger),
		namespace.WithURL(url),
	)
	if err != nil {
		return newErrorNamespaceClient(err)
	}

	// Check if someone else created a client before us.
	m.mu.Lock()
	mapClient, ok := m.nsClients[key]
	if ok {
		client = mapClient
	} else {
		m.nsClients[key] = client
	}
	m.mu.Unlock()

	return client
}

func (m *multiAdminClient) placementClientForCluster(cluster metav1.ObjectMetaAccessor) placement.Client {
	url := m.clusterURLFn(cluster)
	key := m.clusterKeyFn(cluster, url)

	m.mu.RLock()
	client, ok := m.plClients[key]
	m.mu.RUnlock()
	if ok {
		return client
	}

	adminClient := m.adminClientForCluster(cluster)
	client, err := m.plClientFn(
		placement.WithClient(adminClient),
		placement.WithLogger(m.logger),
		placement.WithURL(url),
	)
	if err != nil {
		return newErrorPlacementClient(err)
	}

	m.mu.Lock()
	mapClient, ok := m.plClients[key]
	if ok {
		client = mapClient
	} else {
		m.plClients[key] = client
	}
	m.mu.Unlock()

	return client
}

func (m *multiAdminClient) topicClientForCluster(cluster metav1.ObjectMetaAccessor) topic.Client {
	url := m.clusterURLFn(cluster)
	key := m.clusterKeyFn(cluster, url)

	m.mu.RLock()
	client, ok := m.tpClients[key]
	m.mu.RUnlock()
	if ok {
		return client
	}

	adminClient := m.adminClientForCluster(cluster)
	client, err := m.tpClientFn(
		topic.WithClient(adminClient),
		topic.WithLogger(m.logger),
		topic.WithURL(url),
	)
	if err != nil {
		return newErrorTopicClient(err)
	}

	m.mu.Lock()
	mapClient, ok := m.tpClients[key]
	if ok {
		client = mapClient
	} else {
		m.tpClients[key] = client
	}
	m.mu.Unlock()

	return client
}

// errorNamespaceClient implements namespace.Client by returning an error that a
// specified cluster couldn't be found, enabling easier ergonomics for the
// common pattern of looking up a client and returning an error if one is
// absent.
type errorNamespaceClient struct {
	err error
}

func newErrorNamespaceClient(err error) namespace.Client {
	return errorNamespaceClient{err: err}
}

func (c errorNamespaceClient) Create(request *admin.NamespaceAddRequest) error {
	return c.err
}

func (c errorNamespaceClient) List() (*admin.NamespaceGetResponse, error) {
	return nil, c.err
}

func (c errorNamespaceClient) Delete(namespace string) error {
	return c.err
}

// errorPlacementClient follows the same pattern of errorNamespaceClient for
// placement.Client.
type errorPlacementClient struct {
	err error
}

func newErrorPlacementClient(err error) placement.Client {
	return errorPlacementClient{err: err}
}

func (c errorPlacementClient) Init(request *admin.PlacementInitRequest) error {
	return c.err
}

func (c errorPlacementClient) Get() (placement m3placement.Placement, err error) {
	return nil, c.err
}

func (c errorPlacementClient) Delete() error {
	return c.err
}

func (c errorPlacementClient) Add(placementpb.Instance) error {
	return c.err
}

func (c errorPlacementClient) Remove(string) error {
	return c.err
}

func (c errorPlacementClient) Replace(string, placementpb.Instance) error {
	return c.err
}

type errorTopicClient struct {
	err error
}

func newErrorTopicClient(err error) topic.Client {
	return errorTopicClient{err: err}
}

func (c errorTopicClient) Init(name string, req *admin.TopicInitRequest) error {
	return c.err
}

func (c errorTopicClient) Get(topicName string) (m3topic.Topic, error) {
	return nil, c.err
}

func (c errorTopicClient) Delete(topicName string) error {
	return c.err
}

func (c errorTopicClient) Add(topicName string, consumerSvc *topicpb.ConsumerService) error {
	return c.err
}
