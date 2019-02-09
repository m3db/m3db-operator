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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterConditionType represents the various type of cluster conditions.
type ClusterConditionType string

// IsolationGroups is a slice of IsolationGroup. IsolationGroups satisfies the
// sort.Sort interface, sorting by name.
type IsolationGroups []IsolationGroup

func (g IsolationGroups) Len() int           { return len(g) }
func (g IsolationGroups) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
func (g IsolationGroups) Less(i, j int) bool { return g[i].Name < g[j].Name }

const (
	// ClusterConditionPlacementInitialized indicates an initial placement has
	// been created for the cluster.
	ClusterConditionPlacementInitialized ClusterConditionType = "PlacementInitialized"

	// ClusterConditionPodBootstrapping indicates there is a pod bootstrapping.
	ClusterConditionPodBootstrapping ClusterConditionType = "PodBootstrapping"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// M3DBCluster defines the cluster
type M3DBCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Type              string      `json:"type"`
	Spec              ClusterSpec `json:"spec"`
	Status            M3DBStatus  `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// M3DBClusterList represents a list of M3DB Clusters
type M3DBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []M3DBCluster `json:"items"`
}

// M3DBStatus contains the current state the M3DB cluster along with a human
// readable message
type M3DBStatus struct {
	// State is a enum of green, yellow, and red denoting the health of the
	// cluster
	State M3DBState `json:"state,omitempty"`

	// Various conditions about the cluster.
	Conditions []ClusterCondition `json:"conditions,omitempty"`

	// Message is a human readable message indicating why the cluster is in it's
	// current state
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the last generation of the cluster the controller
	// observed. Kubernetes will automatically increment metadata.Generation every
	// time the cluster spec is changed.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

func (s *M3DBStatus) hasConditionTrue(cond ClusterConditionType) bool {
	for _, c := range s.Conditions {
		if c.Type == cond && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// HasInitializedPlacement returns true if the conditions indicate an initial
// placement has been created.
func (s *M3DBStatus) HasInitializedPlacement() bool {
	return s.hasConditionTrue(ClusterConditionPlacementInitialized)
}

// HasPodBootstrapping returns true if conditions indicate a pod is currently
// bootstrapping.
func (s *M3DBStatus) HasPodBootstrapping() bool {
	return s.hasConditionTrue(ClusterConditionPodBootstrapping)
}

// GetCondition returns the specified cluster condition if it exists with a bool
// indicating whether it was found.
func (s *M3DBStatus) GetCondition(checkCond ClusterConditionType) (ClusterCondition, bool) {
	for _, cond := range s.Conditions {
		if cond.Type == checkCond {
			return cond, true
		}
	}
	return ClusterCondition{}, false
}

// UpdateCondition updates one of the status's conditions, replacing the state
// of cond.Type if it exists or adding the condition if it doesn't exist.
func (s *M3DBStatus) UpdateCondition(newCond ClusterCondition) {
	for i, cond := range s.Conditions {
		if cond.Type == newCond.Type {
			s.Conditions[i] = newCond
			return
		}
	}

	s.Conditions = append(s.Conditions, newCond)
}

// ClusterCondition represents various conditions the cluster can be in.
type ClusterCondition struct {
	// Type of cluster condition.
	Type ClusterConditionType `json:"type,omitempty"`

	// Status of the condition (True, False, Unknown).
	Status corev1.ConditionStatus `json:"status,omitempty"`

	// Last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`

	// Last time this condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`

	// Reason this condition last changed.
	Reason string `json:"reason,omitempty"`

	// Human-friendly message about this condition.
	Message string `json:"message,omitempty"`
}

// M3DBState contains the state of the M3DB cluster
type M3DBState string

const (
	// GreenState indicates a healthy state of the M3DB cluster
	GreenState M3DBState = "green"

	// YellowState indicates a caution state of the M3DB cluster
	YellowState M3DBState = "yellow"

	// RedState indicates a critical state of the M3DB cluster
	RedState M3DBState = "red"
)

// ClusterSpec defines the desired state for a M3 cluster to be converge to.
type ClusterSpec struct {
	// Image specifies which docker image to use with the cluster
	Image string `json:"image,omitempty" yaml:"image"`

	// ReplicationFactor defines how many replicas
	ReplicationFactor int32 `json:"replicationFactor,omitempty" yaml:"replicationFactor"`

	// NumberOfShards defines how many shards in total
	NumberOfShards int32 `json:"numberOfShards,omitempty" yaml:"numberOfShards"`

	// IsolationGroups specifies a map of key-value pairs. Defines which isolation groups
	// to deploy persistent volumes for data nodes
	IsolationGroups []IsolationGroup `json:"isolationGroups,omitempty" yaml:"isolationGroups"`

	// Namespaces specifies the namespaces this cluster will hold.
	Namespaces []Namespace `json:"namespaces,omitempty" yaml:"namespaces"`

	// ConfigMapName specifies the ConfigMap to use for this cluster. If unset a
	// sane default will be used.
	// +optional
	ConfigMapName *string `json:"configMapName,omitempty" yaml:"configMapName"`

	// PodIdentityConfig sets the configuration for pod identity. If unset only
	// pod name and UID will be used.
	// +optional
	PodIdentityConfig *PodIdentityConfig `json:"podIdentityConfig,omitempty" yaml:"podIdentityConfig"`

	// Resources defines memory / cpu constraints for each container in the
	// cluster.
	// +optional
	ContainerResources corev1.ResourceRequirements `json:"containerResources,omitempty" yaml:"containerResources"`

	// DataDirVolumeClaimTemplate is the volume claim template for an M3DB
	// instance's data. It claims PersistentVolumes for cluster storage, volumes
	// are dynamically provisioned by when the StorageClass is defined.
	// +optional
	DataDirVolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"dataDirVolumeClaimTemplate,omitempty" yaml:"dataDirVolumeClaimTemplate"`

	// Labels sets the base labels that will be applied to resources created by
	// the cluster. // TODO(schallert): design doc on labeling scheme.
	Labels map[string]string `json:"labels,omitempty" yaml:"labels"`
}

// IsolationGroup defines the name of zone as well attributes for the zone configuration
type IsolationGroup struct {
	// Name
	Name string `json:"name,omitempty" yaml:"name"`

	// NumInstances defines the number of instances
	NumInstances int32 `json:"numInstances,omitempty" yaml:"numInstances"`
}

// GetByName fetches an IsolationGroup by name.
func (g IsolationGroups) GetByName(name string) (IsolationGroup, bool) {
	for _, group := range g {
		if group.Name == name {
			return group, true
		}
	}
	return IsolationGroup{}, false
}
