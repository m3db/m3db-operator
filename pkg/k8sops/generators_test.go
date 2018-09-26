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
	"testing"

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGenerateCRD(t *testing.T) {
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
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
			Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
				Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
			},
		},
	}

	k, err := newFakeK8sops()
	require.Nil(t, err)
	newCRD := k.GenerateCRD()
	require.Equal(t, crd, newCRD)
}

func TestGenerateService(t *testing.T) {
	fixture := getFixture("testM3DBCluster.yaml", t)
	svcCfg := fixture.Spec.ServiceConfigurations[0]
	k, err := newFakeK8sops()
	require.Nil(t, err)
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   svcCfg.Name,
			Labels: k.GenerateMaps("labels", svcCfg),
		},
		Spec: v1.ServiceSpec{
			Selector:  k.GenerateMaps("selectors", svcCfg),
			Ports:     k.GenerateServicePorts(svcCfg.Ports),
			ClusterIP: "None",
		},
	}
	newSvc := k.GenerateService(svcCfg)
	require.Equal(t, svc, newSvc)
}

func TestGenerateStatefulSet(t *testing.T) {
	fixture := getFixture("testM3DBCluster.yaml", t)
	clusterSpec := fixture.Spec
	svcCfg := fixture.Spec.ServiceConfigurations[0]
	isolationGroup := fixture.Spec.IsolationGroups[0].Name
	instanceAmount := &fixture.Spec.IsolationGroups[0].NumInstances
	clusterName := fixture.GetName()

	k, err := newFakeK8sops()
	require.Nil(t, err)
	ssName := k.StatefulSetName(clusterName, isolationGroup)
	ports := k.GenerateContainerPorts(svcCfg.Ports)

	limitCPU, err := resource.ParseQuantity(clusterSpec.Resources.Limits.CPU)
	require.Nil(t, err)
	require.NotNil(t, limitCPU)

	limitMemory, err := resource.ParseQuantity(clusterSpec.Resources.Limits.Memory)
	require.Nil(t, err)
	require.NotNil(t, limitMemory)

	requestCPU, err := resource.ParseQuantity(clusterSpec.Resources.Requests.CPU)
	require.Nil(t, err)
	require.NotNil(t, requestCPU)

	requestMemory, err := resource.ParseQuantity(clusterSpec.Resources.Requests.Memory)
	require.Nil(t, err)
	require.NotNil(t, requestMemory)

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

	ss := &appsv1.StatefulSet{
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
	newSS, err := k.GenerateStatefulSet(&fixture, clusterSpec, svcCfg, isolationGroup, instanceAmount)
	require.Nil(t, err)
	require.NotNil(t, newSS)
	require.Equal(t, ss, newSS)
}
