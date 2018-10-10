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
	"time"

	"github.com/m3db/m3db-operator/pkg/k8sops"

	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3db-operator/pkg/util/eventer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	_isolationGroupKey = "isolation-group"
	_zoneEmbedded      = "embedded"
)

// Ensure a given namespace exists and update its last updated time. Returns a
// bool which is true if the workqueue should re-enqueue the update (we do this
// if we modify any Kube API objects), and any error encountered.
func (c *Controller) validateNamespaceWithStatus(cluster *myspec.M3DBCluster) (bool, error) {
	created, err := c.EnsureNamespace(cluster)
	if err != nil {
		err = fmt.Errorf("error creating namespace: %v", err)
		c.logger.Error(err.Error())
		c.recorder.WarningEvent(cluster, eventer.ReasonFailedCreate, err.Error())
		return false, err
	}

	if !created && cluster.Status.HasInitializedNamespace() {
		// if we didn't have to create one and the status reflects it's created, do
		// nothing
		return false, nil
	}

	// TODO(schallert): we should use resource version + generation to not
	// update this if not necessary
	cluster.Status.Conditions = append(cluster.Status.Conditions, myspec.ClusterCondition{
		Type:           myspec.ClusterConditionNamespaceInitialized,
		Status:         corev1.ConditionTrue,
		LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
		// TODO(schallert): probably a better reason and we should strongly type
		// them
		Reason:  "NamespaceCreated",
		Message: "Created namespace",
	})

	c.recorder.NormalEvent(cluster, eventer.ReasonSuccessfulCreate, "Namespace created")

	_, err = c.crdClient.OperatorV1().M3DBClusters(cluster.Namespace).Update(cluster)
	if err != nil {
		// TODO(schallert): event here as well?
		err := fmt.Errorf("error updating cluster namespace status: %v", err)
		c.logger.Error(err.Error())
		runtime.HandleError(err)
		return false, err
	}

	// We updated an API object so tell the queue to re-enqueue.
	return true, nil
}

func (c *Controller) validatePlacementWithStatus(cluster *myspec.M3DBCluster) (bool, error) {
	_, err := c.placementClient.Get()
	if err == nil {
		if !cluster.Status.HasInitializedPlacement() {
			// Placement exists but status doesn't reflect that, change it. Return
			// true so event loop re-enqueues.
			return true, c.setStatusPlacementCreated(cluster)
		}

		// Nothing to do, placement already exists and status reflects that
		return false, nil
	}

	if err != m3admin.ErrNotFound {
		err := fmt.Errorf("error from m3admin placement get: %v", err)
		c.logger.Error(err.Error())
		runtime.HandleError(err)
		return false, err
	}

	// Error is just that placement isn't there, let's create it.

	newPlacement := &admin.PlacementInitRequest{
		NumShards:         cluster.Spec.NumberOfShards,
		ReplicationFactor: cluster.Spec.ReplicationFactor,
	}

	serviceName := k8sops.HeadlessServiceName(cluster.Name)
	targetLabels := map[string]string{}
	for k, v := range cluster.Labels {
		targetLabels[k] = v
	}
	// TODO(schallert): extend k8sops helpers for this.
	targetLabels["component"] = "m3dbnode"

	pods, err := c.podLister.Pods(cluster.Namespace).List(labels.SelectorFromSet(targetLabels))
	for _, pod := range pods {
		hostname := pod.Name + "." + serviceName
		instance := &placementpb.Instance{
			Id:             pod.Name,
			IsolationGroup: pod.ObjectMeta.Labels[_isolationGroupKey],
			Zone:           _zoneEmbedded,
			Weight:         100,
			Hostname:       hostname,
			Endpoint:       fmt.Sprintf("%s:%d", hostname, 9000),
			Port:           9000,
		}
		newPlacement.Instances = append(newPlacement.Instances, instance)
	}

	if err := c.placementClient.Init(newPlacement); err != nil {
		return false, err
	}

	return true, nil
}

func (c *Controller) setStatusPlacementCreated(cluster *myspec.M3DBCluster) error {
	cluster.Status.Conditions = append(cluster.Status.Conditions, myspec.ClusterCondition{
		Type:           myspec.ClusterConditionPlacementInitialized,
		Status:         corev1.ConditionTrue,
		LastUpdateTime: time.Now().UTC().Format(time.RFC3339),
		Reason:         "PlacementCreated",
		Message:        "Created placement",
	})

	// TODO(schallert): move to UpdateStatus once 1.10 status subresource is out
	// of alpha.
	_, err := c.crdClient.OperatorV1().M3DBClusters(cluster.Namespace).Update(cluster)
	if err != nil {
		err := fmt.Errorf("error updating cluster placement init status: %v", err)
		c.logger.Error(err.Error())
		runtime.HandleError(err)
		// TODO(schallert): event?
		return err
	}

	return nil
}
