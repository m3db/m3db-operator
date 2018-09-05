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
	"fmt"

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GenerateCRD generates the crd object needed for the M3DBCluster
func (k *K8sops) GenerateCRD() *apiextensionsv1beta1.CustomResourceDefinition {
	return &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: m3dboperator.Name,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   m3dboperator.GroupName,
			Version: m3dboperator.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: m3dboperator.ResourcePlural,
				Kind:   m3dboperator.ResourceKind,
			},
		},
	}
}

const (
	_probeTimeoutSeconds      = 30
	_probeInitialDelaySeconds = 10
	_probeFailureThreshold    = 15
	_probePort                = 7201
	_probePath                = "/health"

	_baseImage              = "quay.io/m3/m3dbnode:latest"
	_dataDirectory          = "/var/lib/m3db/"
	_configurationDirectory = "/etc/m3db/"
	_configurationName      = "m3-configuration"
	_configurationFileName  = "m3.yml"
)

var (
	_configurationFileLocation = fmt.Sprintf("%s%s", _configurationDirectory, _configurationFileName)
)

// GenerateStatefulSet provides a statefulset object for a m3db cluster
func (k *K8sops) GenerateStatefulSet(
	cluster *myspec.M3DBCluster,
	clusterSpec myspec.ClusterSpec,
	svcCfg myspec.ServiceConfiguration,
	isolationGroup string,
	instanceAmount *int32,
) (*appsv1.StatefulSet, error) {
	clusterName := cluster.GetName()
	ssName := k.StatefulSetName(clusterName, isolationGroup)
	ports := k.GenerateContainerPorts(svcCfg.Ports)

	// Parse CPU / Memory
	limitCPU, err := resource.ParseQuantity(clusterSpec.Resources.Limits.CPU)
	if err != nil {
		return nil, err
	}
	limitMemory, err := resource.ParseQuantity(clusterSpec.Resources.Limits.Memory)
	if err != nil {
		return nil, err
	}
	requestCPU, err := resource.ParseQuantity(clusterSpec.Resources.Requests.CPU)
	if err != nil {
		return nil, err
	}
	requestMemory, err := resource.ParseQuantity(clusterSpec.Resources.Requests.Memory)
	if err != nil {
		return nil, err
	}

	probe := &v1.Probe{
		TimeoutSeconds:      _probeTimeoutSeconds,
		InitialDelaySeconds: _probeInitialDelaySeconds,
		FailureThreshold:    _probeFailureThreshold,
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port:   intstr.FromInt(_probePort),
				Path:   _probePath,
				Scheme: v1.URISchemeHTTP,
			},
		},
	}
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: ssName,
			Labels: map[string]string{
				"cluster":        clusterName,
				"app":            "m3dbnode",
				"isolationGroup": isolationGroup,
				"statefulSet":    ssName,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         "m3dbnode",
			PodManagementPolicy: "Parallel",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster":        clusterName,
					"app":            "m3dbnode",
					"isolationGroup": isolationGroup,
					"statefulSet":    ssName,
				},
			},
			Replicas: instanceAmount,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cluster":        clusterName,
						"app":            "m3dbnode",
						"isolationGroup": isolationGroup,
						"statefulSet":    ssName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name: ssName,
							SecurityContext: &v1.SecurityContext{
								Privileged: &[]bool{true}[0],
								Capabilities: &v1.Capabilities{
									Add: []v1.Capability{
										"IPC_LOCK",
									},
								},
							},
							ReadinessProbe: probe,
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
							Ports: ports,
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

// GenerateMaps will produce the proper datastructure for v1.Labels
func (k *K8sops) GenerateMaps(kind string, svcCfg myspec.ServiceConfiguration) map[string]string {
	hash := map[string]string{}
	switch kind {
	case "labels":
		for _, v := range svcCfg.Labels {
			hash[v.Name] = v.Value
		}
	case "selectors":
		for _, v := range svcCfg.Selectors {
			hash[v.Name] = v.Value
		}
	}
	return hash
}

// GenerateService will produce resource configured according to the spec's
// serviceConfiguration fields
func (k *K8sops) GenerateService(svcCfg myspec.ServiceConfiguration) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   svcCfg.Name,
			Labels: k.GenerateMaps("labels", svcCfg),
		},
		Spec: v1.ServiceSpec{
			Selector: k.GenerateMaps("selectors", svcCfg),
			Ports:    k.GenerateServicePorts(svcCfg.Ports),
		},
	}
	if !svcCfg.ClusterIP {
		svc.Spec.ClusterIP = "None"
	}
	return svc
}

// GenerateServicePorts will produce the correct ServicePort or ContainerPort
// resources
func (k *K8sops) GenerateServicePorts(ports []myspec.Port) []v1.ServicePort {
	svcPorts := []v1.ServicePort{}
	for _, v := range ports {
		proto := v1.ProtocolTCP
		if v.Protocol == "udp" {
			proto = v1.ProtocolUDP
		}
		newPortMapping := v1.ServicePort{
			Name:     v.Name,
			Port:     v.Number,
			Protocol: proto,
		}
		svcPorts = append(svcPorts, newPortMapping)
	}
	return svcPorts
}

// GenerateContainerPorts will produce ServicePorts given a
// ServiceConfiguration
func (k *K8sops) GenerateContainerPorts(ports []myspec.Port) []v1.ContainerPort {
	cntPorts := []v1.ContainerPort{}
	for _, v := range ports {
		proto := v1.ProtocolTCP
		if v.Protocol == "udp" {
			proto = v1.ProtocolUDP
		}
		newPortMapping := v1.ContainerPort{
			Name:          v.Name,
			ContainerPort: v.Number,
			Protocol:      proto,
		}
		cntPorts = append(cntPorts, newPortMapping)
	}
	return cntPorts
}
