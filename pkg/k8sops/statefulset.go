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

package k8sops

import (
	errorz "errors"
	"fmt"
	"strings"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// MultiLabelSelector
func (k *K8sops) MultiLabelSelector(kvs map[string]string) metav1.ListOptions {
	var selector string
	for k, v := range kvs {
		selector = fmt.Sprintf("%s %s=%s", selector, k, v)
	}
	selector = strings.Join(strings.Split(strings.TrimRight(selector, " "), " "), ",")
	return metav1.ListOptions{LabelSelector: selector}
}

// LabelSelector
func (k *K8sops) LabelSelector(key, value string) metav1.ListOptions {
	selector := fmt.Sprintf("%s=%s", key, value)
	return metav1.ListOptions{LabelSelector: selector}
}

// DeleteStatefulSets
func (k *K8sops) DeleteStatefuleSets(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) error {
	statefulSets, err := k.GetStatefulSets(cluster, listOpts)
	if err != nil {
		return err
	}
	for _, statefulSet := range statefulSets.Items {
		if err := k.Kclient.
			AppsV1().
			StatefulSets(cluster.GetNamespace()).
			Delete(statefulSet.GetName(), &metav1.DeleteOptions{}); err != nil {
			return err
		}
		k.logger.Info("deleting StatefulSet", zap.String("StatefulSet", statefulSet.GetName()))
	}
	return nil
}

// GetStatefulSets provides all the StatefulSets contained within a
// cluster
func (k *K8sops) GetStatefulSets(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) (*appsv1.StatefulSetList, error) {
	statefulSets, err := k.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).List(listOpts)
	if err != nil {
		return nil, err
	}
	if len(statefulSets.Items) == 0 {
		return nil, errorz.New("failed find any StatefulSets")
	}
	return statefulSets, nil
}

// GetPlacementDetails provides the pod to isolation group mapping
func (k *K8sops) GetPlacementDetails(cluster *myspec.M3DBCluster) (map[string]string, error) {
	placementDetails := make(map[string]string)
	for _, ig := range cluster.Spec.IsolationGroups {
		pods, err := k.GetPodsByLabel(cluster, k.LabelSelector("isolationGroup", ig.Name))
		if err != nil {
			return nil, err
		}
		for _, pod := range pods.Items {
			placementDetails[pod.GetName()] = ig.Name
		}
	}
	return placementDetails, nil
}

// GetPodsByLabel
func (k *K8sops) GetPodsByLabel(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) (*v1.PodList, error) {
	pods, err := k.Kclient.CoreV1().Pods(cluster.GetNamespace()).List(listOpts)
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// EnsureStatefulSets will create StatefulSets based the Spec configuration
// if they don't exist already
func (k *K8sops) EnsureStatefulSets(cluster *myspec.M3DBCluster, svcCfg myspec.ServiceConfiguration) error {
	for _, attrs := range cluster.Spec.IsolationGroups {
		statefulSetName := k.StatefulSetName(cluster.GetName(), attrs.Name)
		statefulSet, err := k.GetStatefulSet(cluster, statefulSetName)
		if errors.IsNotFound(err) {
			k.logger.Info("building statefulset configuration", zap.String("isolationGroup", attrs.Name))
			statefulSet, err = k.GenerateStatefulSet(cluster, cluster.Spec, svcCfg, attrs.Name, &attrs.NumInstances)
			if err != nil {
				k.logger.Error("failed to build statefulset", zap.Error(err))
				return err
			}
			statefulSet, err = k.CreateStatefulSet(cluster, statefulSet)
			if err != nil {
				return err
			}

		} else if errors.IsAlreadyExists(err) {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

// CreateStatefulSet will create a StatefulSet and ensure all Pod replicas are
// ready before returning
func (k *K8sops) CreateStatefulSet(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	statefulSet, err := k.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Create(statefulSet)
	if err != nil {
		k.logger.Error("failed to create statefulset", zap.Error(err), zap.String("statefulset", statefulSet.GetName()))
	}
	statefulSet, err = k.CheckStatefulStatus(cluster, statefulSet)
	if err != nil {
		return nil, err
	}
	k.logger.Info("statefulset created")

	return statefulSet, nil
}

// GetStatefulSet simply returns a StatefulSet given the current cluster
func (k *K8sops) GetStatefulSet(cluster *myspec.M3DBCluster, name string) (*appsv1.StatefulSet, error) {
	statefulSet, err := k.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return statefulSet, nil
}

// UpdateStatefulSet simply updates a statefulset
func (k *K8sops) UpdateStatefulSet(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	statefulSet, err := k.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Update(statefulSet)
	if err != nil {
		return nil, err
	}
	statefulSet, err = k.CheckStatefulStatus(cluster, statefulSet)
	if err != nil {
		return nil, err
	}
	k.logger.Info("updated ss", zap.Any("ss", statefulSet))
	statefulSet, err = k.Kclient.AppsV1().StatefulSets(cluster.GetNamespace()).UpdateStatus(statefulSet)
	if err != nil {
		return nil, err
	}
	k.logger.Info("updated ss status", zap.Any("ss", statefulSet))
	return statefulSet, err
}

// CheckStatefulStatus
func (k *K8sops) CheckStatefulStatus(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	// Poll newly created stateful set and ensure all PODs are in ready state
	err := wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		statefulSet, err := k.GetStatefulSet(cluster, statefulSet.GetName())
		if err != nil {
			return false, err
		}
		if statefulSet.Status.Replicas != statefulSet.Status.ReadyReplicas {
			return false, nil
		}
		k.logger.Info("statefulstate has all replicas in a ready state", zap.Int32("readyReplicas", statefulSet.Status.ReadyReplicas))
		return true, nil
	})
	if err != nil {
		k.logger.Error("ss took longer than 60s to be in ready", zap.Error(err), zap.String("statefulset", statefulSet.GetName()))
		return nil, err
	}
	statefulSet, err = k.GetStatefulSet(cluster, statefulSet.GetName())
	if err != nil {
		return nil, err
	}
	return statefulSet, nil
}
