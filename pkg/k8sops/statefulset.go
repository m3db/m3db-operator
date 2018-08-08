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

package k8sops

import (
	"fmt"

	m3spec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	_probeTimeoutSeconds      = 30
	_probeInitialDelaySeconds = 10
	_probeFailureThreshold    = 15
	_probePort                = 9000
	_probePath                = "/api/v1/health"

	_baseImage              = "quay.io/m3/m3dbnode:latest"
	_dataDirectory          = "/var/lib/m3db/"
	_configurationDirectory = "/etc/m3db/"
	_configurationName      = "m3dbnode-configuration"
	_configurationFileName  = "m3dbnode.yml"
)

var (
	_configurationFileLocation = fmt.Sprintf("%s%s", _configurationDirectory, _configurationFileName)
)

func (k *K8sops) StatefulsetName(clusterName, zone string, replicaFactor int32) string {
	return fmt.Sprintf("%s-%s-%d", clusterName, zone, replicaFactor)
}

// BuildStatefulset provides a statefulset object for a m3db cluster
func (k *K8sops) BuildStatefulset(
	clusterSpec m3spec.ClusterSpec,
	clusterName string,
	namespace string,
	zone string,
	replicas *int32,
	replicaFactor int32,
) (*appsv1.StatefulSet, error) {

	// Parse CPU / Memory
	limitCPU, _ := resource.ParseQuantity(clusterSpec.Resources.Limits.CPU)
	limitMemory, _ := resource.ParseQuantity(clusterSpec.Resources.Limits.Memory)
	requestCPU, _ := resource.ParseQuantity(clusterSpec.Resources.Requests.CPU)
	requestMemory, _ := resource.ParseQuantity(clusterSpec.Resources.Requests.Memory)

	//	probe := &v1.Probe{
	//		TimeoutSeconds:      _probeTimeoutSeconds,
	//		InitialDelaySeconds: _probeInitialDelaySeconds,
	//		FailureThreshold:    _probeFailureThreshold,
	//		Handler: v1.Handler{
	//			HTTPGet: &v1.HTTPGetAction{
	//				Port:   intstr.FromInt(_probePort),
	//				Path:   _probePath,
	//				Scheme: v1.URISchemeHTTP,
	//			},
	//		},
	//	}
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: k.StatefulsetName(clusterName, zone, replicaFactor),
			Labels: map[string]string{
				"cluster":     clusterName,
				"app":         "m3dbnode",
				"statefulset": k.StatefulsetName(clusterName, zone, replicaFactor),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         "m3dbnode",
			PodManagementPolicy: "Parallel",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster":     clusterName,
					"app":         "m3dbnode",
					"statefulset": k.StatefulsetName(clusterName, zone, replicaFactor),
				},
			},
			Replicas: replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cluster":     clusterName,
						"app":         "m3dbnode",
						"statefulset": k.StatefulsetName(clusterName, zone, replicaFactor),
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name: k.StatefulsetName(clusterName, zone, replicaFactor),
							SecurityContext: &v1.SecurityContext{
								Privileged: &[]bool{true}[0],
								Capabilities: &v1.Capabilities{
									Add: []v1.Capability{
										"IPC_LOCK",
									},
								},
							},
							Command: []string{
								"m3dbnode",
							},
							Args: []string{
								"-f",
								_configurationFileLocation,
							},
							Image:           clusterSpec.Image,
							ImagePullPolicy: "Always",
							Env: []v1.EnvVar{
								v1.EnvVar{
									Name: "NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							Ports: k.BuildM3DBNodePorts(),
							VolumeMounts: []v1.VolumeMount{
								v1.VolumeMount{
									Name:      "storage",
									MountPath: _dataDirectory,
								},
								v1.VolumeMount{
									Name:      _configurationName,
									MountPath: _configurationDirectory,
								},
								v1.VolumeMount{
									Name:      "cache",
									MountPath: "/var/lib/m3kv/",
								},
							},
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									"cpu":    limitCPU,
									"memory": limitMemory,
								},
								Requests: v1.ResourceList{
									"cpu":    requestCPU,
									"memory": requestMemory,
								},
							},
						},
					},
					Volumes: []v1.Volume{
						v1.Volume{
							Name: "storage",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
						v1.Volume{
							Name: "cache",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
						v1.Volume{
							Name: _configurationName,
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: _configurationName,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return statefulset, nil
}

// BuildM3DBNodePorts create an array of the default ports of the m3db node service
func (k *K8sops) BuildM3DBNodePorts() []v1.ContainerPort {
	return []v1.ContainerPort{
		v1.ContainerPort{
			Name:          "client",
			ContainerPort: 9200,
			Protocol:      v1.ProtocolTCP,
		},
		v1.ContainerPort{
			Name:          "cluster",
			ContainerPort: 9201,
			Protocol:      v1.ProtocolTCP,
		},
		v1.ContainerPort{
			Name:          "http-node",
			ContainerPort: 9202,
			Protocol:      v1.ProtocolTCP,
		},
		v1.ContainerPort{
			Name:          "debug",
			ContainerPort: 9204,
			Protocol:      v1.ProtocolTCP,
		},
		v1.ContainerPort{
			Name:          "query",
			ContainerPort: 7201,
			Protocol:      v1.ProtocolTCP,
		},
		v1.ContainerPort{
			Name:          "query-metrics",
			ContainerPort: 7203,
			Protocol:      v1.ProtocolTCP,
		},
	}
}

// BuildM3DBCoordinatorPorts create an array of the default ports of the m3db coordinator service
func (k *K8sops) BuildM3DBCoordinatorPorts() []v1.ContainerPort {
	return []v1.ContainerPort{
		v1.ContainerPort{
			Name:          "query",
			ContainerPort: 7201,
			Protocol:      v1.ProtocolTCP,
		},
		v1.ContainerPort{
			Name:          "query-metrics",
			ContainerPort: 7203,
			Protocol:      v1.ProtocolTCP,
		},
	}
}
