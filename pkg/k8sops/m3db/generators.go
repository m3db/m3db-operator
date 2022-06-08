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

package m3db

import (
	"errors"
	"fmt"

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/annotations"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// crdutils "github.com/ant31/crd-validation/pkg"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/utils/pointer"
)

const (
	_probeTimeoutSeconds      = 30
	_probeInitialDelaySeconds = 10
	_probeFailureThreshold    = 15

	_probePathReady = "/bootstrappedinplacementornoplacement"

	_dataDirectory             = "/var/lib/m3db/"
	_dataVolumeName            = "m3db-data"
	_configurationDirectory    = "/etc/m3db/"
	_configurationName         = "m3-configuration"
	_configurationFileLocation = _configurationDirectory + _configurationFileName
	_configurationFileName     = "m3.yml"
	_healthFileName            = "/bin/m3dbnode_bootstrapped.sh"
	_openAPISpecName           = "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1.M3DBCluster"
)

var (
	errEmptyClusterName = errors.New("cluster name cannot be empty")
)

type m3dbPort struct {
	name     string
	port     Port
	protocol v1.Protocol
}

var baseM3DBPorts = [...]m3dbPort{
	{"client", PortM3DBNodeClient, v1.ProtocolTCP},
	{"cluster", PortM3DBNodeCluster, v1.ProtocolTCP},
	{"http-node", PortM3DBHTTPNode, v1.ProtocolTCP},
	{"http-cluster", PortM3DBHTTPCluster, v1.ProtocolTCP},
	{"debug", PortM3DBDebug, v1.ProtocolTCP},
	{"coordinator", PortM3Coordinator, v1.ProtocolTCP},
	{"coord-metrics", PortM3CoordinatorMetrics, v1.ProtocolTCP},
}

var baseCoordinatorPorts = [...]m3dbPort{
	{"coordinator", PortM3Coordinator, v1.ProtocolTCP},
	{"coord-metrics", PortM3CoordinatorMetrics, v1.ProtocolTCP},
}

var carbonListenerPort = m3dbPort{"coord-carbon", PortM3CoordinatorCarbon, v1.ProtocolTCP}

// GenerateCRD generates the crd object needed for the M3DBCluster
func GenerateCRD(enableValidation bool) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: m3dboperator.M3DBClustersName,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: m3dboperator.GroupName,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    m3dboperator.Version,
					Served:  true,
					Storage: true,
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
					},
				},
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: m3dboperator.M3DBClusterResourcePlural,
				Kind:   m3dboperator.M3DBClusterResourceKind,
			},
		},
	}

	// if enableValidation {
	// 	crd.Spec.Versions[0].Schema.openAPIV3Schema = crdutils.GetCustomResourceValidation(_openAPISpecName, myspec.GetOpenAPIDefinitions)
	// }

	return crd
}

// GenerateStatefulSet provides a statefulset object for a m3db cluster
func GenerateStatefulSet(
	cluster *myspec.M3DBCluster,
	isolationGroupName string,
	instanceAmount int32,
) (*appsv1.StatefulSet, error) {

	// TODO(schallert): always sort zones alphabetically.
	stsID := -1
	var isolationGroup myspec.IsolationGroup
	for i, g := range cluster.Spec.IsolationGroups {
		if g.Name == isolationGroupName {
			isolationGroup = g
			stsID = i
			break
		}
	}

	if stsID == -1 {
		return nil, fmt.Errorf("could not find isogroup '%s' in spec", isolationGroupName)
	}

	clusterSpec := cluster.Spec
	clusterName := cluster.GetName()
	ssName := StatefulSetName(clusterName, stsID)

	affinity, err := GenerateStatefulSetAffinity(isolationGroup)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "error generating statefulset affinity")
	}

	statefulSet := NewBaseStatefulSet(ssName, isolationGroupName, cluster, instanceAmount)
	m3dbContainer := &statefulSet.Spec.Template.Spec.Containers[0]
	m3dbContainer.Resources = clusterSpec.ContainerResources
	m3dbContainer.Ports = generateContainerPorts(cluster)
	statefulSet.Spec.Template.Spec.Affinity = affinity
	statefulSet.Spec.Template.Spec.Tolerations = cluster.Spec.Tolerations
	statefulSet.Spec.Template.Spec.HostNetwork = cluster.Spec.HostNetwork
	if cluster.Spec.DNSPolicy != nil {
		statefulSet.Spec.Template.Spec.DNSPolicy = *cluster.Spec.DNSPolicy
	}

	// Set owner ref so sts will be GC'd when the cluster is deleted
	clusterRef := GenerateOwnerRef(cluster)
	statefulSet.OwnerReferences = []metav1.OwnerReference{*clusterRef}

	configVol, configVolMount, err := buildConfigMapComponents(cluster)
	if err != nil {
		return nil, err
	}

	m3dbContainer.VolumeMounts = append(m3dbContainer.VolumeMounts, configVolMount)
	vols := &statefulSet.Spec.Template.Spec.Volumes
	*vols = append(*vols, configVol)

	if cluster.Spec.DataDirVolumeClaimTemplate == nil {
		// No persistent volume claims, add an empty dir for m3db data.
		vols := &statefulSet.Spec.Template.Spec.Volumes
		*vols = append(*vols, v1.Volume{
			Name: _dataVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		})
	} else {
		template := cluster.Spec.DataDirVolumeClaimTemplate.DeepCopy()
		template.ObjectMeta.Name = _dataVolumeName
		if sc := isolationGroup.StorageClassName; sc != "" {
			template.Spec.StorageClassName = pointer.StringPtr(sc)
		}
		statefulSet.Spec.VolumeClaimTemplates = []v1.PersistentVolumeClaim{*template}
	}

	if cluster.Spec.EnvVars != nil && len(cluster.Spec.EnvVars) > 0 {
		cluster := cluster.DeepCopy()
		m3dbContainer.Env = append(m3dbContainer.Env, cluster.Spec.EnvVars...)
	}

	if cluster.Spec.InitContainers != nil && len(cluster.Spec.InitContainers) > 0 {
		cluster := cluster.DeepCopy()
		statefulSet.Spec.Template.Spec.InitContainers = append(statefulSet.Spec.Template.Spec.InitContainers, cluster.Spec.InitContainers...)
	}

	if cluster.Spec.InitVolumes != nil && len(cluster.Spec.InitVolumes) > 0 {
		cluster := cluster.DeepCopy()
		statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, cluster.Spec.InitVolumes...)
	}

	if cluster.Spec.SidecarContainers != nil && len(cluster.Spec.SidecarContainers) > 0 {
		cluster := cluster.DeepCopy()
		statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers, cluster.Spec.SidecarContainers...)
	}

	if cluster.Spec.SidecarVolumes != nil && len(cluster.Spec.SidecarVolumes) > 0 {
		cluster := cluster.DeepCopy()
		statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes, cluster.Spec.SidecarVolumes...)
	}

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
	svcAnnotations := annotations.BaseAnnotations(cluster)
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        HeadlessServiceName(cluster.Name),
			Labels:      svcLabels,
			Annotations: svcAnnotations,
		},
		Spec: v1.ServiceSpec{
			Selector:  svcLabels,
			Ports:     generateM3DBServicePorts(cluster),
			ClusterIP: v1.ClusterIPNone,
			Type:      v1.ServiceTypeClusterIP,
			// Ensure we still publish dbnode DNS names so that we can look up nodes
			// while they're bootstrapping. We don't do this for the coordinator
			// service as we don't want to route unready coordinators.
			PublishNotReadyAddresses: true,
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
	if cluster.Spec.ExternalCoordinator != nil && len(cluster.Spec.ExternalCoordinator.Selector) > 0 {
		selectorLabels = cluster.Spec.ExternalCoordinator.Selector
	}

	serviceLabels := labels.BaseLabels(cluster)
	serviceLabels[labels.Component] = labels.ComponentCoordinator

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   CoordinatorServiceName(cluster.Name),
			Labels: serviceLabels,
		},
		Spec: v1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    generateCoordinatorServicePorts(cluster),
			Type:     v1.ServiceTypeClusterIP,
		},
	}, nil

}

func buildServicePorts(ports []m3dbPort) []v1.ServicePort {
	svcPorts := []v1.ServicePort{}
	for _, p := range ports {
		newPortMapping := v1.ServicePort{
			Name:     p.name,
			Port:     int32(p.port),
			Protocol: p.protocol,
		}
		svcPorts = append(svcPorts, newPortMapping)
	}
	return svcPorts
}

func generateM3DBServicePorts(cluster *myspec.M3DBCluster) []v1.ServicePort {
	ports := baseM3DBPorts[:]
	if cluster.Spec.EnableCarbonIngester {
		ports = append(ports, carbonListenerPort)
	}
	return buildServicePorts(ports)
}

func generateCoordinatorServicePorts(cluster *myspec.M3DBCluster) []v1.ServicePort {
	ports := baseCoordinatorPorts[:]
	if cluster.Spec.EnableCarbonIngester {
		ports = append(ports, carbonListenerPort)
	}
	return buildServicePorts(ports)
}

// generateContainerPorts will produce default container ports.
func generateContainerPorts(cluster *myspec.M3DBCluster) []v1.ContainerPort {
	cntPorts := []v1.ContainerPort{}
	basePorts := baseM3DBPorts[:]
	if cluster.Spec.EnableCarbonIngester {
		basePorts = append(basePorts, carbonListenerPort)
	}
	for _, v := range basePorts {
		newPortMapping := v1.ContainerPort{
			Name:          v.name,
			ContainerPort: int32(v.port),
			Protocol:      v.protocol,
		}
		cntPorts = append(cntPorts, newPortMapping)
	}
	return cntPorts
}
