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
	"errors"
	"fmt"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	podIdentityVolumePath = "/etc/m3db/pod-identity"
	podIdentityVolumeName = "pod-identity"
)

var (
	errEmptyNodeAffinityKey    = errors.New("node affinity term key cannot be empty")
	errEmptyNodeAffinityValues = errors.New("node affinity term values cannot be empty")
)

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
				Port:   intstr.FromInt(PortM3DBHTTPNode),
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
			HTTPGet: &v1.HTTPGetAction{
				Port:   intstr.FromInt(PortM3DBHTTPNode),
				Path:   _probePathReady,
				Scheme: v1.URISchemeHTTP,
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
// given key and values.
func GenerateStatefulSetAffinity(isoGroup myspec.IsolationGroup) (*v1.Affinity, error) {
	if len(isoGroup.NodeAffinityTerms) == 0 {
		return nil, nil
	}

	expressions := make([]v1.NodeSelectorRequirement, len(isoGroup.NodeAffinityTerms))
	for i, term := range isoGroup.NodeAffinityTerms {
		if term.Key == "" {
			return nil, errEmptyNodeAffinityKey
		}
		if len(term.Values) == 0 {
			return nil, errEmptyNodeAffinityValues
		}

		expressions[i] = v1.NodeSelectorRequirement{
			Key:      term.Key,
			Operator: v1.NodeSelectorOpIn,
			Values:   term.Values,
		}
	}

	return &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: expressions,
					},
				},
			},
		},
	}, nil
}

// GenerateOwnerRef generates an owner reference to a given m3db cluster.
func GenerateOwnerRef(cluster *myspec.M3DBCluster) *metav1.OwnerReference {
	return metav1.NewControllerRef(cluster, schema.GroupVersionKind{
		Group:   myspec.SchemeGroupVersion.Group,
		Version: myspec.SchemeGroupVersion.Version,
		Kind:    "m3dbcluster",
	})
}
