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

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	"go.uber.org/zap"
)

const (
	// FailureDomainZoneKey is the standard Kubernetes node label for a zone.
	FailureDomainZoneKey = "failure-domain.beta.kubernetes.io/zone"

	podIdentityVolumePath = "/etc/m3db/pod-identity"
	podIdentityVolumeName = "pod-identity"
)

// MultiLabelSelector provides a ListOptions with a LabelSelector
// given a map of strings
func (k *k8sops) MultiLabelSelector(kvs map[string]string) metav1.ListOptions {
	var selector string
	for k, v := range kvs {
		selector = fmt.Sprintf("%s %s=%s", selector, k, v)
	}
	selector = strings.Join(strings.Split(strings.TrimRight(selector, " "), " "), ",")
	return metav1.ListOptions{LabelSelector: selector}
}

// LabelSelector provides a ListOptions with a LabelSelector given a key
// and value strings
func (k *k8sops) LabelSelector(key, value string) metav1.ListOptions {
	selector := fmt.Sprintf("%s=%s", key, value)
	return metav1.ListOptions{LabelSelector: selector}
}

// DeleteStatefulSets will delete all stateful sets associated with a cluster
func (k *k8sops) DeleteStatefulSets(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) error {
	statefulSets, err := k.GetStatefulSets(cluster, listOpts)
	if err != nil {
		return err
	}
	for _, statefulSet := range statefulSets.Items {
		if err := k.kclient.
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
func (k *k8sops) GetStatefulSets(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) (*appsv1.StatefulSetList, error) {
	statefulSets, err := k.kclient.AppsV1().StatefulSets(cluster.GetNamespace()).List(listOpts)
	if err != nil {
		return nil, err
	}
	if len(statefulSets.Items) == 0 {
		return nil, errorz.New("failed find any StatefulSets")
	}
	return statefulSets, nil
}

// GetPlacementDetails provides the pod to isolation group mapping
func (k *k8sops) GetPlacementDetails(cluster *myspec.M3DBCluster) (map[string]string, error) {
	placementDetails := make(map[string]string)
	for _, ig := range cluster.Spec.IsolationGroups {
		pods, err := k.GetPodsByLabel(cluster, k.LabelSelector(labels.IsolationGroup, ig.Name))
		if err != nil {
			return nil, err
		}
		for _, pod := range pods.Items {
			placementDetails[pod.GetName()] = ig.Name
		}
	}
	return placementDetails, nil
}

// GetPodsByLabel provides a PodList given ListOptions which contain the
// correct LabelSelector
func (k *k8sops) GetPodsByLabel(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) (*v1.PodList, error) {
	pods, err := k.kclient.CoreV1().Pods(cluster.GetNamespace()).List(listOpts)
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// CreateStatefulSet will create a StatefulSet and ensure all Pod replicas are
// ready before returning
func (k *k8sops) CreateStatefulSet(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	statefulSet, err := k.kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Create(statefulSet)
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
func (k *k8sops) GetStatefulSet(cluster *myspec.M3DBCluster, name string) (*appsv1.StatefulSet, error) {
	statefulSet, err := k.kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return statefulSet, nil
}

// UpdateStatefulSet simply updates a statefulset
func (k *k8sops) UpdateStatefulSet(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	statefulSet, err := k.kclient.AppsV1().StatefulSets(cluster.GetNamespace()).Update(statefulSet)
	if err != nil {
		return nil, err
	}
	statefulSet, err = k.CheckStatefulStatus(cluster, statefulSet)
	if err != nil {
		return nil, err
	}
	k.logger.Info("updated ss", zap.Any("ss", statefulSet))
	statefulSet, err = k.kclient.AppsV1().StatefulSets(cluster.GetNamespace()).UpdateStatus(statefulSet)
	if err != nil {
		return nil, err
	}
	k.logger.Info("updated ss status", zap.Any("ss", statefulSet))
	return statefulSet, err
}

// CheckStatefulStatus will poll a given StatefulSet to ensure it reaches a
// ready state within 60 seconds with polling updates at 500 ms
func (k *k8sops) CheckStatefulStatus(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
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

// NewBaseProbe returns a probe configured for default ports.

// NewBaseStatefulSet returns a base configured stateful set.
func NewBaseStatefulSet(ssName, isolationGroup string, cluster *myspec.M3DBCluster, instanceCount int32) *appsv1.StatefulSet {
	ic := instanceCount

	clusterName := cluster.Name
	image := cluster.Spec.Image

	objLabels := labels.BaseLabels(cluster)
	objLabels[labels.IsolationGroup] = isolationGroup
	objLabels[labels.StatefulSet] = ssName
	objLabels[labels.Component] = labels.ComponentM3DBNode
	for k, v := range cluster.Spec.Labels {
		objLabels[k] = v
	}

	// TODO(schallert): we're currently using the health of the coordinator for
	// liveness probes until https://github.com/m3db/m3/issues/996 is fixed. Move
	// to the dbnode's health endpoint once fixed.
	probeHealth := &v1.Probe{
		TimeoutSeconds:      _probeTimeoutSeconds,
		InitialDelaySeconds: _probeInitialDelaySeconds,
		FailureThreshold:    _probeFailureThreshold,
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port:   intstr.FromInt(_probePort),
				Path:   _probePathHealth,
				Scheme: v1.URISchemeHTTP,
			},
		},
	}

	probeReady := &v1.Probe{
		TimeoutSeconds:      _probeTimeoutSeconds,
		InitialDelaySeconds: _probeInitialDelaySeconds,
		FailureThreshold:    _probeFailureThreshold,
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{_healthFileName},
			},
		},
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ssName,
			Labels: objLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: HeadlessServiceName(clusterName),
			Selector: &metav1.LabelSelector{
				MatchLabels: objLabels,
			},
			Replicas: &ic,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objLabels,
				},
				Spec: v1.PodSpec{
					PriorityClassName: cluster.Spec.PriorityClassName,
					SecurityContext:   cluster.Spec.PodSecurityContext,
					Containers: []v1.Container{
						{
							Name:            ssName,
							SecurityContext: cluster.Spec.SecurityContext,
							ReadinessProbe:  probeReady,
							LivenessProbe:   probeHealth,
							Command: []string{
								"m3dbnode",
							},
							Args: []string{
								"-f",
								_configurationFileLocation,
							},
							Image:           image,
							ImagePullPolicy: "Always",
							Env: []v1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							Ports: nil,
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      _dataVolumeName,
									MountPath: _dataDirectory,
								},
								{
									Name:      "cache",
									MountPath: "/var/lib/m3kv/",
								},
								generateDownwardAPIVolumeMount(),
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "cache",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
						generateDownwardAPIVolume(),
					},
				},
			},
		},
	}
}

func generateDownwardAPIVolume() v1.Volume {
	return v1.Volume{
		Name: podIdentityVolumeName,
		VolumeSource: v1.VolumeSource{
			DownwardAPI: &v1.DownwardAPIVolumeSource{
				Items: []v1.DownwardAPIVolumeFile{
					{
						Path: "identity",
						FieldRef: &v1.ObjectFieldSelector{
							FieldPath: fmt.Sprintf("metadata.annotations['%s']", podidentity.AnnotationKeyPodIdentity),
						},
					},
				},
			},
		},
	}
}

func generateDownwardAPIVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      podIdentityVolumeName,
		MountPath: podIdentityVolumePath,
		ReadOnly:  false,
	}
}

// GenerateStatefulSetAffinity generates a node affinity requiring a strict match for
// given key and value.
func GenerateStatefulSetAffinity(isoGroup myspec.IsolationGroup) *v1.Affinity {
	affinityKey := FailureDomainZoneKey
	if k := isoGroup.NodeAffinityKey; k != "" {
		affinityKey = k
	}
	affinityValues := []string{isoGroup.Name}
	if len(isoGroup.NodeAffinityValues) > 0 {
		affinityValues = isoGroup.NodeAffinityValues
	}

	return &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      affinityKey,
								Operator: v1.NodeSelectorOpIn,
								Values:   affinityValues,
							},
						},
					},
				},
			},
		},
	}
}

// GenerateOwnerRef generates an owner reference to a given m3db cluster.
func GenerateOwnerRef(cluster *myspec.M3DBCluster) *metav1.OwnerReference {
	return metav1.NewControllerRef(cluster, schema.GroupVersionKind{
		Group:   myspec.SchemeGroupVersion.Group,
		Version: myspec.SchemeGroupVersion.Version,
		// TODO(schallert): use a const here
		Kind: "m3dbcluster",
	})
}
