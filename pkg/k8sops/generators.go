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

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	_probeTimeoutSeconds      = 30
	_probeInitialDelaySeconds = 10
	_probeFailureThreshold    = 15

	// TODO(schallert): switch to dbnode's endpoint instead of coordinator after
	// https://github.com/m3db/m3/issues/996
	_probePort       = 7201
	_probePathHealth = "/health"

	_dataDirectory          = "/var/lib/m3db/"
	_configurationDirectory = "/etc/m3db/"
	_configurationName      = "m3-configuration"
	_configurationFileName  = "m3.yml"
	_healthFileName         = "/bin/m3dbnode_bootstrapped.sh"
)

var (
	_configurationFileLocation = fmt.Sprintf("%s%s", _configurationDirectory, _configurationFileName)

	errEmptyClusterName = errors.New("cluster name cannot be empty")
)

type m3dbPort struct {
	name     string
	port     int32
	protocol v1.Protocol
}

var baseM3DBPorts = [...]m3dbPort{
	{"client", 9000, v1.ProtocolTCP},
	{"cluster", 9001, v1.ProtocolTCP},
	{"http-node", 9002, v1.ProtocolTCP},
	{"http-cluster", 9003, v1.ProtocolTCP},
	{"debug", 9004, v1.ProtocolTCP},
	{"coordinator", 7201, v1.ProtocolTCP},
	{"coord-metrics", 7203, v1.ProtocolTCP},
}

var baseCoordinatorPorts = [...]m3dbPort{
	{"coordinator", 7201, v1.ProtocolTCP},
	{"coord-metrics", 7203, v1.ProtocolTCP},
}

// GenerateCRD generates the crd object needed for the M3DBCluster
func (k *k8sops) GenerateCRD() *apiextensionsv1beta1.CustomResourceDefinition {
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
			Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
				Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
			},
		},
	}
}

// GenerateStatefulSet provides a statefulset object for a m3db cluster
func GenerateStatefulSet(
	cluster *myspec.M3DBCluster,
	isolationGroup string,
	instanceAmount int32,
) (*appsv1.StatefulSet, error) {

	// TODO(schallert): always sort zones alphabetically.

	stsID := -1
	for i, g := range cluster.Spec.IsolationGroups {
		if g.Name == isolationGroup {
			stsID = i
			break
		}
	}

	if stsID == -1 {
		return nil, fmt.Errorf("could not find isogroup '%s' in spec", isolationGroup)
	}

	clusterName := cluster.GetName()
	ssName := StatefulSetName(clusterName, stsID)

	clusterSpec := cluster.Spec

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

	statefulSet := NewBaseStatefulSet(ssName, isolationGroup, cluster, instanceAmount)
	statefulSet.Spec.Template.Spec.Containers[0].LivenessProbe = probeHealth
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe = probeReady
	statefulSet.Spec.Template.Spec.Containers[0].Resources = clusterSpec.ContainerResources
	statefulSet.Spec.Template.Spec.Containers[0].Ports = generateContainerPorts()
	statefulSet.Spec.Template.Spec.Affinity = GenerateZoneAffinity(isolationGroup)

	// Set owner ref so sts will be GC'd when the cluster is deleted
	clusterRef := GenerateOwnerRef(cluster)
	statefulSet.OwnerReferences = []metav1.OwnerReference{*clusterRef}

	return statefulSet, nil
}

// GenerateM3DBService will generate the headless service required for an M3DB
// StatefulSet.
func GenerateM3DBService(cluster *myspec.M3DBCluster) (*v1.Service, error) {
	if cluster.Name == "" {
		return nil, errEmptyClusterName
	}

	svcLabels := labels.BaseLabels(cluster)
	svcLabels[labels.Component] = labels.ComponentM3DBNode

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   HeadlessServiceName(cluster.Name),
			Labels: svcLabels,
		},
		Spec: v1.ServiceSpec{
			Selector:  svcLabels,
			Ports:     generateM3DBServicePorts(),
			ClusterIP: v1.ClusterIPNone,
			Type:      v1.ServiceTypeClusterIP,
		},
	}, nil
}

// GenerateCoordinatorService creates a coordinator service given a cluster
// name.
func GenerateCoordinatorService(cluster *myspec.M3DBCluster) (*v1.Service, error) {
	if cluster.Name == "" {
		return nil, errEmptyClusterName
	}

	selectorLabels := labels.BaseLabels(cluster)
	selectorLabels[labels.Component] = labels.ComponentM3DBNode

	serviceLabels := labels.BaseLabels(cluster)
	serviceLabels[labels.Component] = labels.ComponentCoordinator

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   CoordinatorServiceName(cluster.Name),
			Labels: serviceLabels,
		},
		Spec: v1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    generateCoordinatorServicePorts(),
			Type:     v1.ServiceTypeClusterIP,
		},
	}, nil
}

func buildServicePorts(ports []m3dbPort) []v1.ServicePort {
	svcPorts := []v1.ServicePort{}
	for _, p := range ports {
		newPortMapping := v1.ServicePort{
			Name:     p.name,
			Port:     p.port,
			Protocol: p.protocol,
		}
		svcPorts = append(svcPorts, newPortMapping)
	}
	return svcPorts
}

func generateM3DBServicePorts() []v1.ServicePort {
	return buildServicePorts(baseM3DBPorts[:])
}

func generateCoordinatorServicePorts() []v1.ServicePort {
	return buildServicePorts(baseCoordinatorPorts[:])
}

// generateContainerPorts will produce default container ports.
func generateContainerPorts() []v1.ContainerPort {
	cntPorts := []v1.ContainerPort{}
	for _, v := range baseM3DBPorts {
		newPortMapping := v1.ContainerPort{
			Name:          v.name,
			ContainerPort: v.port,
			Protocol:      v.protocol,
		}
		cntPorts = append(cntPorts, newPortMapping)
	}
	return cntPorts
}
