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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/placement"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"

	"go.uber.org/zap"
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
	cluster.Status.UpdateCondition(myspec.ClusterCondition{
		Type:           myspec.ClusterConditionNamespaceInitialized,
		Status:         corev1.ConditionTrue,
		LastUpdateTime: c.clock.Now().UTC().Format(time.RFC3339),
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

	targetLabels := map[string]string{}
	for k, v := range cluster.Labels {
		targetLabels[k] = v
	}
	targetLabels[labels.Component] = labels.ComponentM3DBNode

	sel := klabels.SelectorFromSet(targetLabels)
	c.logger.Debug("placement init selector", zap.String("selector", sel.String()))
	pods, err := c.podLister.Pods(cluster.Namespace).List(sel)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		instance, err := k8sops.PlacementInstanceFromPod(cluster, pod)
		if err != nil {
			return false, err
		}

		newPlacement.Instances = append(newPlacement.Instances, instance)
	}

	if err := c.placementClient.Init(newPlacement); err != nil {
		return false, err
	}

	return true, nil
}

func (c *Controller) setStatusPlacementCreated(cluster *myspec.M3DBCluster) error {
	cluster.Status.UpdateCondition(myspec.ClusterCondition{
		Type:           myspec.ClusterConditionPlacementInitialized,
		Status:         corev1.ConditionTrue,
		LastUpdateTime: c.clock.Now().UTC().Format(time.RFC3339),
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

func (c *Controller) setStatusPodBootstrapping(cluster *myspec.M3DBCluster,
	status corev1.ConditionStatus,
	reason, message string) (*myspec.M3DBCluster, error) {

	return c.setStatus(cluster, myspec.ClusterConditionPodBootstrapping, status, reason, message)
}

func (c *Controller) setStatus(cluster *myspec.M3DBCluster, condition myspec.ClusterConditionType,
	status corev1.ConditionStatus, reason, message string) (*myspec.M3DBCluster, error) {

	cond, ok := cluster.Status.GetCondition(condition)
	if !ok {
		cond = myspec.ClusterCondition{
			Type:   condition,
			Status: corev1.ConditionUnknown,
		}
	}

	curTime := c.clock.Now().UTC().Format(time.RFC3339)
	if cond.Status != status {
		cond.LastTransitionTime = curTime
	}

	cond.Status = status
	cond.LastUpdateTime = curTime
	cond.Reason = reason
	cond.Message = message
	cluster.Status.UpdateCondition(cond)

	return c.crdClient.OperatorV1().M3DBClusters(cluster.Namespace).Update(cluster)
}

// Updates the cluster if there had been a condition that a pod was
// bootstrapping but no pods are currently bootstrapping.
func (c *Controller) reconcileBootstrappingStatus(cluster *myspec.M3DBCluster, placement placement.Placement) (*myspec.M3DBCluster, error) {
	for _, inst := range placement.Instances() {
		if !inst.IsAvailable() {
			return cluster, nil
		}
	}

	return c.setStatus(cluster, myspec.ClusterConditionPodBootstrapping, corev1.ConditionFalse,
		"BootstrapComplete", "no bootstraps in progress")
}

func (c *Controller) addPodToPlacement(cluster *myspec.M3DBCluster, pod *corev1.Pod, placement placement.Placement) error {
	c.logger.Info("found pod not in placement", zap.String("pod", pod.Name))
	inst, err := k8sops.PlacementInstanceFromPod(cluster, pod)
	if err != nil {
		err := fmt.Errorf("error creating instance for pod %s", pod.Name)
		c.logger.Error(err.Error())
		return err
	}

	reason := fmt.Sprintf("adding pod %s to placement", pod.Name)
	_, err = c.setStatusPodBootstrapping(cluster, corev1.ConditionTrue, "PodAdded", reason)
	if err != nil {
		err := fmt.Errorf("error setting pod bootstrapping status: %v", err)
		c.logger.Error(err.Error())
		return err
	}

	err = c.placementClient.Add(*inst)
	if err != nil {
		err := fmt.Errorf("error adding pod to placement: %s", pod.Name)
		c.logger.Error(err.Error())
		return err
	}

	c.logger.Info("added pod to placement", zap.String("pod", pod.Name))
	return nil
}

// expandPlacementForSet takes a StatefulSet that has pods in it which need to
// be added to the placement and chooses a pod to expand to the placement.
func (c *Controller) expandPlacementForSet(cluster *myspec.M3DBCluster, set *appsv1.StatefulSet,
	group myspec.IsolationGroup, placement placement.Placement) error {

	existInsts := instancesInIsoGroup(placement, group.Name)
	if len(existInsts) >= int(group.NumInstances) {
		c.logger.Warn("not expanding set, already at desired capacity",
			zap.Int32("groupSize", group.NumInstances),
			zap.Int("instsInGroup", len(existInsts)))
		return nil
	}

	if set.Status.ReadyReplicas < group.NumInstances {
		c.logger.Error("cannot expand set, ready replicas < desired",
			zap.Int32("ready", set.Status.ReadyReplicas),
			zap.Int32("desired", group.NumInstances))
		return fmt.Errorf("cannot expand set '%s', not yet ready", set.Name)
	}

	selector := klabels.SelectorFromSet(set.Labels)
	pods, err := c.podLister.Pods(cluster.Namespace).List(selector)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		_, ok := placement.Instance(pod.Name)
		if !ok {
			return c.addPodToPlacement(cluster, pod, placement)
		}
	}

	return errors.New("could not find pod absent from placement")
}

// shrinkPlacementForSet takes a StatefulSet that needs to be shrunk and
// removes the last pod in the StatefulSet from the active placement, enabling
// the StatefulSet size to be decreased once the remove completes.
func (c *Controller) shrinkPlacementForSet(cluster *myspec.M3DBCluster, set *appsv1.StatefulSet) error {
	selector := klabels.SelectorFromSet(set.Labels)
	pods, err := c.podLister.Pods(cluster.Namespace).List(selector)
	if err != nil {
		c.logger.Error("error listing pods", zap.Error(err))
		return err
	}

	removePod, err := findPodToRemove(pods)
	if err != nil {
		c.logger.Error("error finding pod to remove", zap.Error(err))
		return err
	}

	c.logger.Info("removing pod from placement", zap.String("pod", removePod.Name))
	return c.placementClient.Remove(removePod.Name)
}

// podID encapsulates a pod and its ordinal ID to facilitate sorting a list of
// pod names by ID and easily keeping a reference to the original pod.
type podID struct {
	pod *corev1.Pod
	id  int
}

// byPodID supports sorting a list of statefulset pods by their ordinal ID.
type byPodID []podID

func (names byPodID) Len() int           { return len(names) }
func (names byPodID) Swap(i, j int)      { names[i], names[j] = names[j], names[i] }
func (names byPodID) Less(i, j int) bool { return names[i].id < names[j].id }

// findPodToRemove returns the pod name with the highest ordinal number in the
// stateful set so that we remove from the placement the pod that will be
// deleted when the set size is scaled down.
func findPodToRemove(pods []*corev1.Pod) (*corev1.Pod, error) {
	if len(pods) == 0 {
		return nil, errors.New("cannot find removal candidate in empty list")
	}

	podIDs := make([]podID, len(pods))
	for i, pod := range pods {
		parts := strings.Split(pod.Name, "-")
		if len(parts) == 0 {
			return nil, fmt.Errorf("invalid pod name '%s'", pod.Name)
		}

		id := parts[len(parts)-1]
		idN, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("error parsing pod '%s' ID: %v", pod.Name, err)
		}

		podIDs[i] = podID{
			pod: pod,
			id:  idN,
		}
	}

	sort.Sort(byPodID(podIDs))
	lastPod := podIDs[len(podIDs)-1].pod

	return lastPod, nil
}
