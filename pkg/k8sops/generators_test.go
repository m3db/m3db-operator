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
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGenerateStatefulSet(t *testing.T) {
	fixture := getFixture("testM3DBCluster.yaml", t)
	clusterSpec := fixture.Spec
	isolationGroup := fixture.Spec.IsolationGroups[0].Name
	instanceAmount := &fixture.Spec.IsolationGroups[0].NumInstances
	clusterName := fixture.GetName()

	ssName := StatefulSetName(clusterName, 0)

	health := &v1.Probe{
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

	readiness := &v1.Probe{
		TimeoutSeconds:      _probeTimeoutSeconds,
		InitialDelaySeconds: _probeInitialDelaySeconds,
		FailureThreshold:    _probeFailureThreshold,
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{_healthFileName},
			},
		},
	}

	labels := map[string]string{
		"operator.m3db.io/cluster":         clusterName,
		"operator.m3db.io/app":             "m3db",
		"operator.m3db.io/component":       "m3dbnode",
		"operator.m3db.io/stateful-set":    ssName,
		"operator.m3db.io/isolation-group": "us-fake1-a",
	}

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ssName,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(fixture, schema.GroupVersionKind{
					Group:   myspec.SchemeGroupVersion.Group,
					Version: myspec.SchemeGroupVersion.Version,
					Kind:    "m3dbcluster",
				}),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         "m3dbnode-m3db-cluster",
			PodManagementPolicy: "Parallel",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instanceAmount,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      FailureDomainZoneKey,
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{isolationGroup},
											},
										},
									},
								},
							},
						},
					},
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
							LivenessProbe:  health,
							ReadinessProbe: readiness,
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
							Ports: generateContainerPorts(),
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
									"cpu":    resource.MustParse("2"),
									"memory": resource.MustParse("2Gi"),
								},
								Requests: v1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
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

	newSS, err := GenerateStatefulSet(fixture, isolationGroup, *instanceAmount)
	require.Nil(t, err)
	require.NotNil(t, newSS)

	require.Equal(t, ss, newSS)
}

func TestGenerateM3DBService(t *testing.T) {
	cluster := &myspec.M3DBCluster{}
	svc, err := GenerateM3DBService(cluster)
	assert.Error(t, err)
	assert.Nil(t, svc)

	cluster.Name = "cluster-a"
	svc, err = GenerateM3DBService(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	baseLabels := map[string]string{
		labels.Cluster:   cluster.Name,
		labels.App:       labels.AppM3DB,
		labels.Component: labels.ComponentM3DBNode,
	}

	expSvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "m3dbnode-cluster-a",
			Labels: baseLabels,
		},
		Spec: v1.ServiceSpec{
			Selector:  baseLabels,
			Ports:     generateM3DBServicePorts(),
			ClusterIP: v1.ClusterIPNone,
			Type:      v1.ServiceTypeClusterIP,
		},
	}

	assert.Equal(t, expSvc, svc)
}

func TestGenerateCoordinatorService(t *testing.T) {
	cluster := &myspec.M3DBCluster{}
	svc, err := GenerateCoordinatorService(cluster)
	assert.Error(t, err)
	assert.Nil(t, svc)

	cluster.Name = "cluster-a"
	svc, err = GenerateCoordinatorService(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	selectLabels := map[string]string{
		labels.Cluster:   cluster.Name,
		labels.App:       labels.AppM3DB,
		labels.Component: labels.ComponentM3DBNode,
	}

	svcLabels := map[string]string{
		labels.Cluster:   cluster.Name,
		labels.App:       labels.AppM3DB,
		labels.Component: labels.ComponentCoordinator,
	}

	expSvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "m3coordinator-cluster-a",
			Labels: svcLabels,
		},
		Spec: v1.ServiceSpec{
			Selector: selectLabels,
			Ports:    generateCoordinatorServicePorts(),
			Type:     v1.ServiceTypeClusterIP,
		},
	}

	assert.Equal(t, expSvc, svc)
}
