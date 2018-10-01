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

	containerPorts := []v1.ContainerPort(nil)

	// TODO(schallert): fix this
	if len(cluster.Spec.ServiceConfigurations) > 0 {
		containerPorts = GenerateContainerPorts(cluster.Spec.ServiceConfigurations[0].Ports)
	}

	clusterSpec := cluster.Spec
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

	resources := GenerateResourceRequirements(limitCPU, limitMemory, requestCPU, requestMemory)
	statefulSet := NewBaseStatefulSet(ssName, clusterSpec.Image, clusterName, isolationGroup, instanceAmount)
	statefulSet.Spec.Template.Spec.Containers[0].ReadinessProbe = probe
	statefulSet.Spec.Template.Spec.Containers[0].Resources = resources
	statefulSet.Spec.Template.Spec.Containers[0].Ports = containerPorts
	statefulSet.Spec.Template.Spec.Affinity = GenerateZoneAffinity(isolationGroup)

	// Set owner ref so sts will be GC'd when the cluster is deleted
	clusterRef := GenerateOwnerRef(cluster)
	statefulSet.OwnerReferences = []metav1.OwnerReference{*clusterRef}

	return statefulSet, nil
}

// GenerateMaps will produce the proper datastructure for v1.Labels
func GenerateMaps(kind string, svcCfg myspec.ServiceConfiguration) map[string]string {
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
// serviceConfiguration fields.
func GenerateService(svcCfg myspec.ServiceConfiguration) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   svcCfg.Name,
			Labels: GenerateMaps("labels", svcCfg),
		},
		Spec: v1.ServiceSpec{
			Selector: GenerateMaps("selectors", svcCfg),
			Ports:    GenerateServicePorts(svcCfg.Ports),
		},
	}
	if !svcCfg.ClusterIP {
		svc.Spec.ClusterIP = "None"
	}
	return svc
}

// GenerateServicePorts will produce the correct ServicePort or ContainerPort
// resources.
func GenerateServicePorts(ports []myspec.Port) []v1.ServicePort {
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

// GenerateContainerPorts will produce ServicePorts given a ServiceConfiguration.
func GenerateContainerPorts(ports []myspec.Port) []v1.ContainerPort {
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

// GenerateResourceRequirements creates a base resource requirement.
func GenerateResourceRequirements(limitCPU, limitMemory, requestCPU, requestMemory resource.Quantity) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    limitCPU,
			"memory": limitMemory,
		},
		Requests: v1.ResourceList{
			"cpu":    requestCPU,
			"memory": requestMemory,
		},
	}
}
