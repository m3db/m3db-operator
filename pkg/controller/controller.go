// Copyright (c) 2016 Uber Technologies, Inc.
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
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/Sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// reconcilerLock ensures that reconciliation and event reconciling does
// not happen at the same time.
var (
	reconcilerLock = &sync.Mutex{}
)

// Cluster is a map of active M3DB clusters
type Cluster struct {
	M3DBCluster *myspec.M3DBCluster
}

// Controller object
type Controller struct {
	k8sclient *k8sops.K8sops
	clusters  map[string]Cluster
	m3admin   m3admin.M3admin
}

// New creates new instance of Reconcile
func New(kclient *k8sops.K8sops) (*Controller, error) {
	p := &Controller{
		k8sclient: kclient,
		clusters:  make(map[string]Cluster),
		m3admin:   m3admin.New(),
	}

	return p, nil
}

// Init ensures all the required resources are created
func (c *Controller) Init() error {
	if err := c.k8sclient.CreateKubernetesCustomResourceDefinition(); err != nil {
		return err
	}
	if err := c.refreshClusters(); err != nil {
		logrus.WithError(err).Error("failed to refresh clusters")
		return err
	}

	logrus.Infof("found %d existing clusters ", len(c.clusters))
	return nil
}

// Start controling?
func (c *Controller) Start(done chan struct{}, wg *sync.WaitGroup) error {

	// Watch for events that add, modify, or delete M3DBCluster definitions andlog
	// process them asynchronously.
	logrus.Info("watching for m3db events")
	events, watchErrs := c.k8sclient.MonitorM3DBEvents(done)
	go func() {
		for {
			select {
			case event := <-events:
				if err := c.reconcileM3DBClusterEvent(event); err != nil {
					logrus.WithError(err).Error("failed to reconcile")
				}
			case err := <-watchErrs:
				logrus.WithError(err).Error("watch errors occured")
			case <-done:
				wg.Done()
				logrus.Info("stopped m3db event watcher.")
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
		logrus.WithError(err).Error("could not get list of clusters")
		return err
	}

	for _, cluster := range currentClusters.Items {
		logrus.Infof("found cluster: %s", cluster.ObjectMeta.Name)
		currCluster := cluster.DeepCopy()
		c.clusters[currCluster.ObjectMeta.Name] = Cluster{M3DBCluster: currCluster}
	}

	return nil
}

func (c *Controller) reconcileM3DBClusterEvent(cluster *myspec.M3DBCluster) error {
	reconcilerLock.Lock()
	defer reconcilerLock.Unlock()
	logrus.WithField("event", cluster.Type).Info("received M3DB cluster event")
	switch {
	case cluster.Type == "ADDED":
		if err := c.k8sclient.EnsureM3DBNodeSvc(cluster); err != nil {
			return err
		}
		if err := c.k8sclient.EnsureM3CoordinatorSvc(cluster); err != nil {
			return err
		}
		return c.addM3DBCluster(cluster)
	case cluster.Type == "MODIFIED":
		//return c.updateM3DBCluster(cluster)
		return nil
	case cluster.Type == "DELETED":
		if err := c.deleteM3DBCluster(cluster); err != nil {
			return err
		}
		if err := c.k8sclient.DeleteM3DBNodeSvc(cluster); err != nil {
			return err
		}
		return c.k8sclient.DeleteM3CoordinatorSvc(cluster)
	}
	return nil
}

func (c *Controller) updateM3DBCluster(cluster *myspec.M3DBCluster) error {

	oldClusters := make(map[string]Cluster)
	for clusterName, cluster := range c.clusters {
		oldClusters[clusterName] = Cluster{M3DBCluster: cluster.M3DBCluster.DeepCopy()}
	}

	if err := c.refreshClusters(); err != nil {
		logrus.WithError(err).Error("failed to refresh cluster, can not compare modified state")
		return err
	}

	if oldClusters[cluster.ObjectMeta.Name].M3DBCluster.Spec.Image != c.clusters[cluster.ObjectMeta.Name].M3DBCluster.Spec.Image {
		if err := c.deployNewImage(cluster); err != nil {
			logrus.WithError(err).Error("failed to deploy new image")
			return err
		}
	}
	return nil
}

func (c *Controller) deployNewImage(cluster *myspec.M3DBCluster) error {
	//_, _ = c.k8sclient.Kclient.AppsV1().Statefulsets(cluster.GetNamespace()).Update()
	return nil
}

func (c *Controller) addM3DBCluster(cluster *myspec.M3DBCluster) error {
	// Refresh cluster map with latest state
	if err := c.refreshClusters(); err != nil {
		logrus.WithError(err).Error("error refreshing cluster")
		return err
	}
	placementDetails := make(map[string]string) // key is pod name, value zone

	// ensure zones are present in spec
	if len(cluster.Spec.Zones) != 0 {

		// TODO(PS) add zone distribution algo to range
		// loop through zones
		for zoneIndex, zoneReplicaCount := range []int32{1, 1, 1} {

			// Create statefulsets
			logrus.WithFields(logrus.Fields{
				"zone":          cluster.Spec.Zones[zoneIndex],
				"replicasCount": zoneReplicaCount}).
				Info("building statefulset configuration")
			statefulset, err := c.k8sclient.BuildStatefulset(
				cluster.Spec,
				cluster.ObjectMeta.Name,
				cluster.ObjectMeta.Namespace,
				cluster.Spec.Zones[zoneIndex],
				&zoneReplicaCount,
				cluster.Spec.ReplicationFactor,
			)
			if err != nil {
				logrus.WithError(err).Error("failed to build statefulset")
				return err
			}

			if statefulset, err := c.k8sclient.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Create(statefulset); err != nil {
				logrus.WithError(err).WithField("statefulset", statefulset).Error("failed to create statefulset")
			}
			logrus.Info("statefulset created")

			// Poll newly created stateful set and ensure all PODs are in ready state
			err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
				createdSS, err := c.k8sclient.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Get(statefulset.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if createdSS.Status.ReadyReplicas != cluster.Spec.ReplicationFactor {
					return false, nil
				}
				logrus.WithField("readyReplicas", createdSS.Status.ReadyReplicas).Info("statefulstate has all replicas in a ready state")
				return true, nil
			})
			if err != nil {
				logrus.WithError(err).WithField("statefulset", statefulset.GetName()).Error("ss took longer than 60s to be in ready")
				return err
			}
			selector := fmt.Sprintf("statefulset=%s", statefulset.GetName())
			createdPods, err := c.k8sclient.Kclient.CoreV1().Pods(cluster.GetNamespace()).List(metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				return err
			}

			// Add each POD to map by zone
			for _, pod := range createdPods.Items {
				placementDetails[pod.GetName()] = cluster.Spec.Zones[zoneIndex]
			}
		}
	}

	//create namespace
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		if err := c.m3admin.NamespaceCreate(cluster); err != nil {
			logrus.WithError(err).Error("failed to create namespace, trying again")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logrus.WithError(err).Error("failed to create namespace after 6 tries")
		return err
	}

	//create placement
	err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		if err := c.m3admin.PlacementInit(cluster, placementDetails); err != nil {
			logrus.WithError(err).Error("failed to init placement, trying again")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logrus.WithError(err).Error("failed to init placement after 6 tries")
		return err
	}
	return nil
}

func (c *Controller) deleteM3DBCluster(cluster *myspec.M3DBCluster) error {
	if err := c.m3admin.PlacementDelete(cluster); err != nil {
		return err
	}
	if err := c.m3admin.NamespaceDelete(cluster, "default"); err != nil {
		return err
	}
	for _, zone := range cluster.Spec.Zones {
		statefulsetName := c.k8sclient.StatefulsetName(cluster.ObjectMeta.Name, zone, 1)
		logrus.WithField("statefulsetName", statefulsetName).Info("deleting stateful set")
		if err := c.k8sclient.Kclient.AppsV1().
			StatefulSets(cluster.GetNamespace()).
			Delete(statefulsetName, &metav1.DeleteOptions{}); err != nil {
			logrus.WithError(err).WithField("statefulsetName", statefulsetName).Error("failed to delete statefulset")
		}
	}
	return nil
}
