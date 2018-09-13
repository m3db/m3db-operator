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
	"sync"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"

	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcilerLock ensures that reconciliation and event reconciling does
// not happen at the same time.
var (
	reconcilerLock              = &sync.Mutex{}
	ErrIsolationGroupsMissing   = errors.New("no isolation groups  specified")
	ErrInvalidReplicationFactor = errors.New("invalid replication factor")
)

// Cluster contains the CRD for the M3 cluster
type Cluster struct {
	M3DBCluster *myspec.M3DBCluster
}

// Controller object
type Controller struct {
	logger          *zap.Logger
	k8sclient       *k8sops.K8sops
	clusters        map[string]Cluster
	placementClient placement.Placement
	namespaceClient namespace.Namespace
}

// New creates new instance of Controller
func New(logger *zap.Logger, kclient *k8sops.K8sops) (*Controller, error) {
	p := &Controller{
		logger:    logger,
		k8sclient: kclient,
		clusters:  make(map[string]Cluster),
	}
	// TODO(PS) Move these clients within the cluster object to esnure each
	// cluster has it's own configured client
	placementClient, err := placement.NewClient(placement.WithLogger(logger))
	if err != nil {
		return nil, err
	}
	p.placementClient = placementClient
	namespaceClient, err := namespace.NewClient(namespace.WithLogger(logger))
	if err != nil {
		return nil, err
	}
	p.namespaceClient = namespaceClient
	return p, nil
}

// Init ensures all the required resources are created
func (c *Controller) Init() error {
	if err := c.k8sclient.CreateKubernetesCustomResourceDefinition(); err != nil {
		return err
	}
	if err := c.refreshClusters(); err != nil {
		c.logger.Error("failed to refresh clusters", zap.Error(err))
		return err
	}
	c.logger.Info("found existing", zap.Int("clusters", len(c.clusters)))
	return nil
}

// Start the controller
func (c *Controller) Start(done chan struct{}, wg *sync.WaitGroup) error {

	// Watch for events that add, modify, or delete M3DBCluster definitions andlog
	// process them asynchronously.
	c.logger.Info("watching for m3db events")
	events, watchErrs := c.k8sclient.MonitorM3DBEvents(done)
	go func() {
		for {
			select {
			case event := <-events:
				if err := c.reconcileM3DBClusterEvent(event); err != nil {
					c.logger.Error("failed to reconcile", zap.Error(err))
				}
			case err := <-watchErrs:
				c.logger.Error("watch errors occured", zap.Error(err))
			case <-done:
				wg.Done()
				c.logger.Info("stopped m3db event watcher.")
				return
			}
		}
	}()
	return nil
}

func (c *Controller) refreshClusters() error {

	//Reset
	c.clusters = make(map[string]Cluster)

	// Get existing clusters
	currentClusters, err := c.k8sclient.CrdClient.OperatorV1().M3DBClusters(v1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		c.logger.Error("could not get list of clusters", zap.Error(err))
		return err
	}

	for _, cluster := range currentClusters.Items {
		c.logger.Info("found cluster", zap.String("name", cluster.ObjectMeta.Name))
		currCluster := cluster.DeepCopy()
		c.clusters[currCluster.ObjectMeta.Name] = Cluster{M3DBCluster: currCluster}
	}

	return nil
}

func (c *Controller) validateClusterSpec(cluster *myspec.M3DBCluster) error {
	// TODO(PS) LoadM3Configuration when dep conflicts are resolved. The
	// configuration will allow us to validate the port numbers specified in the
	// m3 configuration align with the ones within the M3DBCluster spec.
	/*
		m3Config, err := c.LoadM3Configuration(cluster)
		if err != nil {
			return err
		}
	*/

	// ensure zones are present in spec
	if len(cluster.Spec.IsolationGroups) == 0 {
		c.logger.Error("isolationGroups missing from spec", zap.Error(ErrIsolationGroupsMissing))
		return ErrIsolationGroupsMissing
	}
	if cluster.Spec.ReplicationFactor > int32(len(cluster.Spec.IsolationGroups)) {
		c.logger.Error("replication factor is greater than zones (failure domains)", zap.Error(ErrInvalidReplicationFactor))
		return ErrInvalidReplicationFactor
	}
	return nil
}

func (c *Controller) reconcileM3DBClusterEvent(cluster *myspec.M3DBCluster) error {
	reconcilerLock.Lock()
	defer reconcilerLock.Unlock()
	// TODO(PS) Replace with validation within spec or use validation package
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/core/validation/validation.go
	if err := c.validateClusterSpec(cluster); err != nil {
		return err
	}
	c.logger.Info("received M3DB cluster event", zap.String("event", cluster.Type))
	switch {
	case cluster.Type == "ADDED":
		return c.addM3DBCluster(cluster)
	case cluster.Type == "MODIFIED":
		return c.updateM3DBCluster(cluster)
	case cluster.Type == "DELETED":
		return c.deleteM3DBCluster(cluster)
	}
	return nil
}

func (c *Controller) deployNewImage(cluster *myspec.M3DBCluster) error {
	//_, _ = c.k8sclient.Kclient.AppsV1().Statefulsets(cluster.GetNamespace()).Update()
	return nil
}
