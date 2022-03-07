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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	"github.com/m3db/m3/src/cluster/placement"
	dbns "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"

	pkgerrors "github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	errEmptyPodList         = errors.New("cannot find removal candidate in empty list")
	errNoPodsInPlacement    = errors.New("no pods were found in the placement")
	errNilNamespaceRegistry = errors.New("nil registry for namespaces")
	errPodNotInPlacement    = errors.New("instance not found in placement")
)

// reconcileNamespaces will delete any namespaces currently in the cluster that
// aren't part of the cluster spec, and create any that are present in the spec
// but not in the cluster.
func (c *M3DBController) reconcileNamespaces(cluster *myspec.M3DBCluster) error {
	resp, err := c.adminClient.namespaceClientForCluster(cluster).List()
	if err != nil {
		c.logger.Error("failed to get namespace", zap.Error(err))
		c.recorder.WarningEvent(cluster, eventer.ReasonFailSync, err.Error())
		return err
	}

	if err := c.pruneNamespaces(cluster, resp.Registry); err != nil {
		return err
	}

	if err := c.createNamespaces(cluster, resp.Registry); err != nil {
		return err
	}

	return c.readyNamespaces(cluster, resp.Registry)
}

// createNamespaces will attempt to create in the cluster all namespaces which
// are present in the spec but not the cluster.
func (c *M3DBController) createNamespaces(cluster *myspec.M3DBCluster, registry *dbns.Registry) error {
	toCreate := namespacesToCreate(registry, cluster.Spec.Namespaces)
	for _, ns := range toCreate {
		req, err := namespace.RequestFromSpec(ns)
		if err != nil {
			c.logger.Error("error forming namespace request",
				zap.String("namespace", ns.Name),
				zap.Error(err))

			return fmt.Errorf("error forming request for namespace '%s': %v", ns.Name, err)
		}

		err = c.adminClient.namespaceClientForCluster(cluster).Create(req)
		if err != nil {
			c.logger.Error("error creating namespace",
				zap.String("namespace", ns.Name),
				zap.Error(err))

			return fmt.Errorf("error creating namespace '%s': %w", ns.Name, err)
		}

		c.recorder.NormalEvent(cluster, eventer.ReasonCreating, "created namespace "+ns.Name)
	}

	return nil
}

// readyNamespaces will attempt to mark all namespaces in the initializing state as ready.
func (c *M3DBController) readyNamespaces(cluster *myspec.M3DBCluster, registry *dbns.Registry) error {
	// NB(nate): Marking namespaces ready (without force) involves checking dbnodes for namespace
	// availability. External coordinators do not have connections to dbnodes so don't attempt this
	// step for now.
	if cluster.Spec.ExternalCoordinator != nil {
		c.logger.Debug("using an external coordinator. will not attempt to ready namespaces")
		return nil
	}

	toReady := namespacesToReady(registry, cluster.Spec.Namespaces)
	for _, ns := range toReady {
		req := &admin.NamespaceReadyRequest{
			Name: ns.Name,
		}
		err := c.adminClient.namespaceClientForCluster(cluster).Ready(req)
		cause := pkgerrors.Cause(err)
		// NB(nate): Due to bug in coordinator API routing logic, missing routes may
		// also be returned as 405s (i.e. method not allowed). So check for that in addition
		// to 404s.
		if cause == m3admin.ErrNotFound || cause == m3admin.ErrMethodNotAllowed {
			c.logger.Info("coordinator does not yet support the ready endpoint. " +
				"skipping readying namespaces until upgraded")
			return nil
		}
		if err != nil {
			c.logger.Error("error readying namespace",
				zap.String("namespace", ns.Name),
				zap.Error(err))

			return fmt.Errorf("error readying namespace '%s': %v", ns.Name, err)
		}
	}

	return nil
}

// pruneNamespaces will delete any namespaces in the m3db cluster that aren't
// in the spec.
func (c *M3DBController) pruneNamespaces(cluster *myspec.M3DBCluster, registry *dbns.Registry) error {
	toDelete := namespacesToDelete(registry, cluster.Spec.Namespaces)
	for _, ns := range toDelete {
		err := c.adminClient.namespaceClientForCluster(cluster).Delete(ns)
		if err == nil {
			c.logger.Info("deleted namespace", zap.String("namespace", ns))
			c.recorder.NormalEvent(cluster, eventer.ReasonDeleting, "deleted namespace "+ns)
			continue
		}
		// TODO(nate): Set the StagingStatus to initializing here once we're guaranteed that each
		// coordinator can support receiving the JSON field.

		if pkgerrors.Cause(err) == m3admin.ErrNotFound {
			c.logger.Info("namespace has already been deleted", zap.String("namespace", ns))
			continue
		}

		c.logger.Error("error deleting namespace",
			zap.String("namespace", ns),
			zap.Error(err))
		return err
	}

	return nil
}

// namespacesToCreate returns an array of namespaces that are in the cluster
// spec but not in the registry.
func namespacesToCreate(registry *dbns.Registry, specNs []myspec.Namespace) (toCreate []myspec.Namespace) {
	for _, ns := range specNs {
		if _, ok := registry.Namespaces[ns.Name]; !ok {
			toCreate = append(toCreate, ns)
		}
	}
	return
}

// namespacesToDelete returns an array of namespace names that are in the
// registry but not in the cluster spec.
func namespacesToDelete(registry *dbns.Registry, specNs []myspec.Namespace) (toDelete []string) {
	inSpec := make(map[string]struct{})
	for _, ns := range specNs {
		inSpec[ns.Name] = struct{}{}
	}

	// If any namespace is in the registry but not in the spec, we want to delete
	// it.
	for ns := range registry.Namespaces {
		if _, ok := inSpec[ns]; !ok {
			toDelete = append(toDelete, ns)
		}
	}

	return
}

// namespacesToReady returns an array of namespaces that are in the initializing state.
func namespacesToReady(registry *dbns.Registry, specNs []myspec.Namespace) (toReady []myspec.Namespace) {
	namespacesToReady := make(map[string]myspec.Namespace)
	// Add namespaces we've just created.
	for _, ns := range specNs {
		if _, ok := registry.Namespaces[ns.Name]; !ok {
			namespacesToReady[ns.Name] = ns
		}
	}

	// Add any existing namespaces we've found not in the ready state.
	for name, options := range registry.Namespaces {
		if options.StagingState != nil && options.StagingState.Status != dbns.StagingStatus_READY {
			namespacesToReady[name] = myspec.Namespace{Name: name}
		}
	}

	for _, ns := range namespacesToReady {
		toReady = append(toReady, ns)
	}

	return
}

func (c *M3DBController) validatePlacementWithStatus(
	ctx context.Context, cluster *myspec.M3DBCluster,
) (*myspec.M3DBCluster, error) {
	plClient := c.adminClient.placementClientForCluster(cluster)
	_, err := plClient.Get()
	if err == nil {
		if !cluster.Status.HasInitializedPlacement() {
			return c.setStatusPlacementCreated(ctx, cluster)
		}

		// Nothing to do, placement already exists and status reflects that
		return cluster, nil
	}

	if pkgerrors.Cause(err) != m3admin.ErrNotFound {
		err := fmt.Errorf("error from m3admin placement get: %v", err)
		c.logger.Error(err.Error())
		runtime.HandleError(err)
		return nil, err
	}

	// Error is just that placement isn't there, let's create it.

	newPlacement := &admin.PlacementInitRequest{
		NumShards:         cluster.Spec.NumberOfShards,
		ReplicationFactor: cluster.Spec.ReplicationFactor,
	}

	targetLabels := labels.BaseLabels(cluster)
	for k, v := range cluster.Spec.Labels {
		targetLabels[k] = v
	}
	targetLabels[labels.Component] = labels.ComponentM3DBNode

	sel := klabels.SelectorFromSet(targetLabels)
	c.logger.Debug("placement init selector", zap.String("selector", sel.String()))
	pods, err := c.podLister.Pods(cluster.Namespace).List(sel)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods {
		instance, err := m3db.PlacementInstanceFromPod(cluster, pod, c.podIDProvider)
		if err != nil {
			return nil, err
		}

		newPlacement.Instances = append(newPlacement.Instances, instance)
	}

	if err := plClient.Init(newPlacement); err != nil {
		return nil, err
	}

	return c.setStatusPlacementCreated(ctx, cluster)
}

func (c *M3DBController) setStatusPlacementCreated(
	ctx context.Context, cluster *myspec.M3DBCluster,
) (*myspec.M3DBCluster, error) {
	cluster.Status.UpdateCondition(myspec.ClusterCondition{
		Type:           myspec.ClusterConditionPlacementInitialized,
		Status:         corev1.ConditionTrue,
		LastUpdateTime: c.clock.Now().UTC().Format(time.RFC3339),
		Reason:         "PlacementCreated",
		Message:        "Created placement",
	})

	var err error
	cluster, err = c.crdClient.OperatorV1alpha1().M3DBClusters(cluster.Namespace).
		UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
	if err != nil {
		c.logger.Error("error updating cluster placement init status",
			zap.Error(err))
		c.recorder.WarningEvent(cluster, eventer.ReasonFailSync, "failed to update placement status: %v", err)
		return nil, err
	}

	c.logger.Info("updated cluster placement status", zap.String("cluster", cluster.Name),
		zap.Any("status", cluster.Status))
	return cluster, nil
}

func (c *M3DBController) setStatusPodsBootstrapping(
	ctx context.Context,
	cluster *myspec.M3DBCluster,
	status corev1.ConditionStatus,
	reason, message string,
) (*myspec.M3DBCluster, error) {
	return c.setStatus(ctx, cluster, myspec.ClusterConditionPodsBootstrapping, status, reason, message)
}

func (c *M3DBController) setStatus(
	ctx context.Context,
	cluster *myspec.M3DBCluster,
	condition myspec.ClusterConditionType,
	status corev1.ConditionStatus,
	reason, message string,
) (*myspec.M3DBCluster, error) {
	cond, ok := cluster.Status.GetCondition(condition)
	if !ok {
		cond = myspec.ClusterCondition{
			Type:   condition,
			Status: corev1.ConditionUnknown,
		}
	}

	if ok && cond.Status == status {
		c.logger.Debug("conditions equal, nothing to do",
			zap.String("condition", string(condition)),
			zap.String("status", string(status)),
		)
		return cluster, nil
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

	return c.crdClient.OperatorV1alpha1().M3DBClusters(cluster.Namespace).
		UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
}

// Updates the cluster if there had been a condition that a pod was
// bootstrapping but no pods are currently bootstrapping.
func (c *M3DBController) reconcileBootstrappingStatus(
	ctx context.Context, cluster *myspec.M3DBCluster, placement placement.Placement,
) (*myspec.M3DBCluster, error) {
	for _, inst := range placement.Instances() {
		if !inst.IsAvailable() {
			return cluster, nil
		}
	}

	return c.setStatus(ctx, cluster, myspec.ClusterConditionPodsBootstrapping, corev1.ConditionFalse,
		"BootstrapComplete", "no bootstraps in progress")
}

func (c *M3DBController) addPodsToPlacement(
	ctx context.Context, cluster *myspec.M3DBCluster, pods []*corev1.Pod,
) error {
	var (
		instances = make([]*placementpb.Instance, 0, len(pods))
		podNames  = make([]string, 0, len(pods))
	)
	for _, pod := range pods {
		c.logger.Info("found pod not in placement", zap.String("pod", pod.Name))
		inst, err := m3db.PlacementInstanceFromPod(cluster, pod, c.podIDProvider)
		if err != nil {
			c.logger.Error("error creating instance for pod",
				zap.String("pod", pod.Name),
				zap.Error(err))
			return err
		}
		instances = append(instances, inst)
		podNames = append(podNames, pod.Name)
	}
	reason := fmt.Sprintf("adding pods to placement (%s)", strings.Join(podNames, ", "))
	_, err := c.setStatusPodsBootstrapping(ctx, cluster, corev1.ConditionTrue, "PodAdded", reason)
	if err != nil {
		c.logger.Error("error setting pods bootstrapping status",
			zap.Error(err))
		return err
	}

	err = c.adminClient.placementClientForCluster(cluster).Add(instances)
	if err != nil {
		c.logger.Error("error adding instances to placement",
			zap.String("reason", reason),
			zap.Error(err))
		return err
	}

	c.logger.Info("added pods to placement", zap.Strings("pods", podNames))
	return nil
}

func (c *M3DBController) checkPodsForReplacement(
	cluster *myspec.M3DBCluster,
	pods []*corev1.Pod,
	pl placement.Placement) (string, *corev1.Pod, error) {
	insts := pl.Instances()
	sort.Sort(placement.ByIDAscending(insts))

	sortedPods, err := sortPods(pods)
	if err != nil {
		return "", nil, fmt.Errorf("cannot sort pods: %v", err)
	}

	for _, pod := range sortedPods {
		clusterPodID, err := c.podIDProvider.Identity(pod.pod, cluster)
		if err != nil {
			return "", nil, err
		}

		for _, inst := range insts {
			var instancePodID myspec.PodIdentity

			if strings.EqualFold(strings.Split(inst.Hostname(), ".")[0], pod.pod.Name) {
				if err = json.Unmarshal([]byte(inst.ID()), &instancePodID); err != nil {
					return "", nil, err
				}

				if !reflect.DeepEqual(*clusterPodID, instancePodID) {
					return inst.ID(), pod.pod, nil
				}
			}
		}
	}

	return "", nil, nil
}

func (c *M3DBController) replacePodInPlacement(
	ctx context.Context,
	cluster *myspec.M3DBCluster,
	pl placement.Placement,
	leavingInstanceID string,
	newPod *corev1.Pod,
) error {
	c.logger.Info("replacing pod in placement", zap.String("pod", leavingInstanceID))

	newInst, err := m3db.PlacementInstanceFromPod(cluster, newPod, c.podIDProvider)
	if err != nil {
		c.logger.Error("error creating instance from replacement pod",
			zap.String("pod", newPod.Name),
			zap.Error(err))
		return err
	}

	reason := fmt.Sprintf("replacing %s pod in placement", newPod.Name)
	_, err = c.setStatusPodsBootstrapping(ctx, cluster, corev1.ConditionTrue, "PodReplaced", reason)
	if err != nil {
		c.logger.Error("error setting replacement pod bootstrapping status",
			zap.Error(err))
		return err
	}

	err = c.adminClient.placementClientForCluster(cluster).Replace(leavingInstanceID, *newInst)
	if err != nil {
		c.logger.Error("error replacing pod in placement",
			zap.Error(err))
		return err
	}

	return nil
}

// expandPlacementForSet takes a StatefulSet that has pods in it which need to
// be added to the placement and chooses a pod to expand to the placement.
func (c *M3DBController) expandPlacementForSet(
	ctx context.Context,
	cluster *myspec.M3DBCluster,
	set *appsv1.StatefulSet,
	group myspec.IsolationGroup, //nolint:gocritic
	placement placement.Placement,
) error {
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

	podsToAdd := make([]*corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		id, err := c.podIDProvider.Identity(pod, cluster)
		if err != nil {
			return err
		}
		idStr, err := podidentity.IdentityJSON(id)
		if err != nil {
			return err
		}
		_, ok := placement.Instance(idStr)
		if !ok {
			podsToAdd = append(podsToAdd, pod)
		}
	}
	if len(podsToAdd) == 0 {
		return errors.New("could not find pod absent from placement")
	}

	return c.addPodsToPlacement(ctx, cluster, podsToAdd)
}

// shrinkPlacementForSet takes a StatefulSet that needs to be shrunk and
// removes the last pod in the StatefulSet from the active placement, enabling
// the StatefulSet size to be decreased once the remove completes.
func (c *M3DBController) shrinkPlacementForSet(
	cluster *myspec.M3DBCluster, set *appsv1.StatefulSet, pl placement.Placement, removeCount int,
) error {
	if cluster.Spec.PreventScaleDown {
		return pkgerrors.Errorf("cannot remove nodes from %s/%s, preventScaleDown is true",
			cluster.Namespace, cluster.Name)
	}

	selector := klabels.SelectorFromSet(set.Labels)
	pods, err := c.podLister.Pods(cluster.Namespace).List(selector)
	if err != nil {
		c.logger.Error("error listing pods", zap.Error(err))
		return err
	}

	_, removeInst, err := c.findPodInstancesToRemove(cluster, pl, pods, removeCount)
	if err != nil {
		c.logger.Error("error finding pod to remove", zap.Error(err))
		return err
	}

	if len(removeInst) == 0 {
		c.logger.Info("nothing to remove, skipping remove call")
		return nil
	}

	var removeIds []string
	for _, inst := range removeInst {
		removeIds = append(removeIds, inst.ID())
	}
	c.logger.Info("removing pods from placement", zap.String("instances", strings.Join(removeIds, ",")))
	return c.adminClient.placementClientForCluster(cluster).Remove(removeIds)
}

// findPodInstancesToRemove returns the pod (and associated placement instace)
// with the highest ordinal number in the stateful set AND in the placement, so
// that we remove from the placement the pod that will be deleted when the set
// size is scaled down.
func (c *M3DBController) findPodInstancesToRemove(
	cluster *myspec.M3DBCluster,
	pl placement.Placement,
	pods []*corev1.Pod,
	removeCount int,
) ([]*corev1.Pod, []placement.Instance, error) {
	if len(pods) == 0 {
		return nil, nil, errEmptyPodList
	}

	podIDs, err := sortPods(pods)
	if err != nil {
		return nil, nil, pkgerrors.WithMessage(err, "cannot sort pods")
	}

	var (
		podsToRemove      []*corev1.Pod
		instancesToRemove []placement.Instance
		leftToRemove      = removeCount
	)
	for i := len(podIDs) - 1; i >= 0 && leftToRemove > 0; i-- {
		pod := podIDs[i].pod
		inst, err := c.findPodInPlacement(cluster, pl, pod)
		if pkgerrors.Cause(err) == errPodNotInPlacement {
			// If the instance is already out of the placement, continue to the next
			// one.
			continue
		}
		if err != nil {
			return nil, nil, pkgerrors.WithMessage(err, "error finding pod in placement")
		}
		leftToRemove -= 1
		podsToRemove = append(podsToRemove, pod)
		instancesToRemove = append(instancesToRemove, inst)
	}
	return podsToRemove, instancesToRemove, nil
}

// findPodInPlacement looks up a pod in the placement. Equality is based on
// whether a pods identity matches a placement instance's ID.
func (c *M3DBController) findPodInPlacement(
	cluster *myspec.M3DBCluster, pl placement.Placement, pod *corev1.Pod,
) (placement.Instance, error) {
	id, err := c.podIDProvider.Identity(pod, cluster)
	if err != nil {
		return nil, err
	}
	idStr, err := podidentity.IdentityJSON(id)
	if err != nil {
		return nil, err
	}
	inst, ok := pl.Instance(idStr)
	if !ok {
		return nil, errPodNotInPlacement
	}
	return inst, nil
}

func (c *M3DBController) updateFinalizers(
	ctx context.Context, cluster *myspec.M3DBCluster,
) (*myspec.M3DBCluster, error) {
	var err error
	cluster, err = c.crdClient.OperatorV1alpha1().M3DBClusters(cluster.Namespace).
		Update(ctx, cluster, metav1.UpdateOptions{})
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "error updating cluster finalizers")
	}
	return cluster, nil
}

// ensureEtcdFinalizer ensures that the etcd deletion finalizer is present.
func (c *M3DBController) ensureEtcdFinalizer(
	ctx context.Context, cluster *myspec.M3DBCluster,
) (*myspec.M3DBCluster, error) {
	if stringArrayContains(cluster.Finalizers, labels.EtcdDeletionFinalizer) {
		return cluster, nil
	}

	c.logger.Info("adding etcd finalizer to cluster", zap.String("cluster", cluster.Name))
	cluster.ObjectMeta.Finalizers = append(cluster.ObjectMeta.Finalizers, labels.EtcdDeletionFinalizer)
	return c.updateFinalizers(ctx, cluster)
}

// removeEtcdFinalizer ensures the etcd finalizer is absent.
func (c *M3DBController) removeEtcdFinalizer(
	ctx context.Context, cluster *myspec.M3DBCluster,
) (*myspec.M3DBCluster, error) {
	if !stringArrayContains(cluster.Finalizers, labels.EtcdDeletionFinalizer) {
		return cluster, nil
	}

	finalizers := make([]string, 0, len(cluster.Finalizers))
	for _, f := range cluster.Finalizers {
		if f != labels.EtcdDeletionFinalizer {
			finalizers = append(finalizers, f)
		}
	}

	cluster.Finalizers = finalizers
	return c.updateFinalizers(ctx, cluster)
}

func (c *M3DBController) deleteAllNamespaces(cluster *myspec.M3DBCluster) error {
	clusterLogger := c.logger.With(zap.String("cluster", cluster.Name))
	clusterLogger.Info("cleaning up cluster namespaces")

	nsClient := c.adminClient.namespaceClientForCluster(cluster)
	namespaces, err := nsClient.List()
	if err != nil {
		return pkgerrors.WithMessage(err, "error listing namespaces for deletion")
	}
	if namespaces.Registry == nil {
		return errNilNamespaceRegistry
	}

	for name := range namespaces.Registry.Namespaces {
		if err := nsClient.Delete(name); err != nil {
			return pkgerrors.WithMessagef(err, "error deleting namespace %s", name)
		}
		clusterLogger.Info("deleted namespace during cleanup", zap.String("namespace", name))
	}

	return nil
}

func (c *M3DBController) deletePlacement(cluster *myspec.M3DBCluster) error {
	clusterLogger := c.logger.With(zap.String("cluster", cluster.Name))
	clusterLogger.Info("cleaning up cluster placement")

	// If the placement doesn't exist there's no need to delete it. Can remove
	// the initial Get() once https://github.com/m3db/m3/pull/1701 is merged and
	// in a release.
	plClient := c.adminClient.placementClientForCluster(cluster)
	_, err := plClient.Get()
	if err != nil {
		// If the placement is not found there's nothing to do.
		if pkgerrors.Cause(err) == m3admin.ErrNotFound {
			return nil
		}
		return pkgerrors.WithMessage(err, "error fetching placement to delete")
	}

	if err := plClient.Delete(); err != nil {
		return pkgerrors.WithMessagef(err, "error deleting placement for cluster %s", cluster.Name)
	}

	clusterLogger.Info("deleted cluster placement")
	return nil
}

func stringArrayContains(arr []string, s string) bool {
	for _, str := range arr {
		if str == s {
			return true
		}
	}
	return false
}

func sortPods(pods []*corev1.Pod) ([]podID, error) {
	if pods == nil {
		return nil, errEmptyPodList
	}

	podIDs := make([]podID, len(pods))
	for i, pod := range pods {
		parts := strings.Split(pod.Name, "-")
		if len(parts) == 0 {
			return nil, fmt.Errorf("invalid pod name: '%s'", pod.Name)
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
	return podIDs, nil
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
