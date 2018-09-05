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
	"time"

	"github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"
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
	k8sclient       k8sops.K8sops
	clusters        map[string]Cluster
	placementClient placement.Client
	namespaceClient namespace.Client
	doneCh          chan struct{}
}

// New creates new instance of Controller
func New(logger *zap.Logger, kclient k8sops.K8sops) (*Controller, error) {
	p := &Controller{
		lock:      &sync.Mutex{},
		logger:    logger,
		k8sclient: kclient,
		clusters:  make(map[string]Cluster),
		doneCh:    make(chan struct{}),
	}
	// TODO(PS) Move these clients within the cluster object to ensure each
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

// Stop will stop the controller
func (c *Controller) Stop() {
	close(c.doneCh)
}

// Init ensures all the required resources are created
func (c *Controller) Init() error {
	if err := c.k8sclient.CreateCRD(m3dboperator.Name); err != nil {
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
func (c *Controller) Start(wg *sync.WaitGroup) error {
	// Watch for events that add, modify, or delete M3DBCluster definitions andlog
	// process them asynchronously.
	c.logger.Info("watching for m3db events")
	events, watchErrs := c.monitorM3DBEvents(c.doneCh)
	go func() {
		for {
			select {
			case event := <-events:
				if err := c.reconcileM3DBClusterEvent(event); err != nil {
					c.logger.Error("failed to reconcile", zap.Error(err))
				}
			case err := <-watchErrs:
				c.logger.Error("watch errors occured", zap.Error(err))
			case <-c.doneCh:
				wg.Done()
				c.logger.Info("stopped m3db event watcher.")
				return
			}
		}
	}()
	return nil
}

// monitorM3DBEvents watches for new or removed clusters
func (c *Controller) monitorM3DBEvents(stopchan chan struct{}) (<-chan *myspec.M3DBCluster, <-chan error) {
	events := make(chan *myspec.M3DBCluster)
	errc := make(chan error, 1)
	source := c.k8sclient.NewListWatcher()

	createAddHandler := func(obj interface{}) {
		event := obj.(*myspec.M3DBCluster)
		event.Type = "ADDED"
		events <- event
	}

	createDeleteHandler := func(obj interface{}) {
		event := obj.(*myspec.M3DBCluster)
		event.Type = "DELETED"
		events <- event
	}

	updateHandler := func(old interface{}, obj interface{}) {
		event := obj.(*myspec.M3DBCluster)
		event.Type = "MODIFIED"
		events <- event
	}

	_, controller := cache.NewInformer(
		source,
		&myspec.M3DBCluster{},
		time.Minute*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    createAddHandler,
			UpdateFunc: updateHandler,
			DeleteFunc: createDeleteHandler,
		})

	go controller.Run(stopchan)

	return events, errc
}
func (c *Controller) refreshClusters() error {

	//Reset
	c.clusters = make(map[string]Cluster)

	// Get active clusters
	currentClusters, err := c.k8sclient.ListM3DBCluster()
	if err != nil {
		return err
	}

	// Copy objects into Cluster object
	for _, cluster := range currentClusters.Items {
		c.logger.Info("found cluster", zap.String("name", cluster.ObjectMeta.Name))
		currCluster := cluster.DeepCopy()
		c.clusters[currCluster.ObjectMeta.Name] = Cluster{M3DBCluster: currCluster}
	}

	return nil
}

func (c *Controller) validateClusterSpec(cluster *myspec.M3DBCluster) error {

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
	c.lock.Lock()
	defer c.lock.Unlock()
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
