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
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type K8sops interface {
	// StatefulSetName provides a formatted string to use for naming StatefulSets
	StatefulSetName(clusterName, isolationGroup string) string

	// ListM3DBCluster will list all the CRDS for M3DBClusters in all namespaces
	ListM3DBCluster() (*myspec.M3DBClusterList, error)

	// GetM3DBCluster will get the M3DBCluster CRD
	GetM3DBCluster(namespace, name string) (*myspec.M3DBCluster, error)

	// GetCRD will get a CRD
	GetCRD(name string) (*apiextensionsv1beta1.CustomResourceDefinition, error)

	// CreateCRD checks if M3DB CRD exists. If not, create
	CreateCRD(name string) error

	// GenerateCRD generates the crd object needed for the M3DBCluster
	GenerateCRD() *apiextensionsv1beta1.CustomResourceDefinition

	// UpdateCRD will update a CRD
	UpdateCRD(cluster *myspec.M3DBCluster) (*myspec.M3DBCluster, error)

	// NewListWatcher will provide a list watcher for M3DB objects
	NewListWatcher() *cache.ListWatch

	// GenerateStatefulSet provides a statefulset object for a m3db cluster
	GenerateStatefulSet(
		cluster *myspec.M3DBCluster,
		clusterSpec myspec.ClusterSpec,
		svcCfg myspec.ServiceConfiguration,
		isolationGroup string,
		instanceAmount *int32,
	) (*appsv1.StatefulSet, error)

	// GenerateMaps will produce the proper datastructure for v1.Labels
	GenerateMaps(kind string, svcCfg myspec.ServiceConfiguration) map[string]string

	// GenerateService will produce resource configured according to the spec's
	// serviceConfiguration fields
	GenerateService(svcCfg myspec.ServiceConfiguration) *v1.Service

	// GenerateServicePorts will produce the correct ServicePort or ContainerPort
	// resources
	GenerateServicePorts(ports []myspec.Port) []v1.ServicePort

	// GenerateContainerPorts will produce ServicePorts given a
	// ServiceConfiguration
	GenerateContainerPorts(ports []myspec.Port) []v1.ContainerPort

	// GetService simply gets a service by name
	GetService(cluster *myspec.M3DBCluster, name string) (*v1.Service, error)

	// DeleteService simply deletes a service by name
	DeleteService(cluster *myspec.M3DBCluster, name string) error

	// EnsureService will create a service by name if it doesn't exist
	EnsureService(cluster *myspec.M3DBCluster, svcCfg myspec.ServiceConfiguration) error

	// MultiLabelSelector provides a ListOptions with a LabelSelector
	// given a map of strings
	MultiLabelSelector(kvs map[string]string) metav1.ListOptions

	// LabelSelector provides a ListOptions with a LabelSelector given a key
	// and value strings
	LabelSelector(key, value string) metav1.ListOptions

	// DeleteStatefulSets will delete all stateful sets associated with a cluster
	DeleteStatefulSets(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) error

	// GetStatefulSets provides all the StatefulSets contained within a
	// cluster
	GetStatefulSets(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) (*appsv1.StatefulSetList, error)

	// GetPlacementDetails provides the pod to isolation group mapping
	GetPlacementDetails(cluster *myspec.M3DBCluster) (map[string]string, error)

	// GetPodsByLabel provides a PodList given ListOptions which contain the
	// correct LabelSelector
	GetPodsByLabel(cluster *myspec.M3DBCluster, listOpts metav1.ListOptions) (*v1.PodList, error)

	// EnsureStatefulSets will create StatefulSets based the Spec configuration
	// if they don't exist already
	EnsureStatefulSets(cluster *myspec.M3DBCluster, svcCfg myspec.ServiceConfiguration) error

	// CreateStatefulSet will create a StatefulSet and ensure all Pod replicas are
	// ready before returning
	CreateStatefulSet(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)

	// GetStatefulSet simply returns a StatefulSet given the current cluster
	GetStatefulSet(cluster *myspec.M3DBCluster, name string) (*appsv1.StatefulSet, error)

	// UpdateStatefulSet simply updates a statefulset
	UpdateStatefulSet(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)

	// CheckStatefulStatus will poll a given StatefulSet to ensure it reaches a
	// ready state within configurable amount of time
	CheckStatefulStatus(cluster *myspec.M3DBCluster, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error)

	// GetConfigMap retrieves a config by name
	GetConfigMap(cluster *myspec.M3DBCluster, name string) (*v1.ConfigMap, error)
}
