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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Message is a human readable message indicating why the cluster is in it's
	// current state
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
	Image string `json:"image" yaml:"image"`

	// ReplicationFactor defines how many replicas
	ReplicationFactor int32 `json:"replicationFactor" yaml:"replicationFactor"`

	// NumberOfShards defines how many shards in total
	NumberOfShards int32 `json:"numberOfShards" yaml:"numberOfShards"`

	// IsolationGroups specifies a map of key-value pairs. Defines which isolation groups
	// to deploy persistent volumes for data nodes
	IsolationGroups []IsolationGroup `json:"isolationGroups" yaml:"isolationGroups"`

	// Resources defines memory / cpu constraints
	Resources Resources `json:"resources" yaml:"resources"`

	// ServiceConfiguration contains the requested service configurations
	ServiceConfigurations []ServiceConfiguration `json:"serviceConfigurations" yaml:"serviceConfigurations"`
}

// Label is meta to reference the resource by
type Label struct {
	// Name of the label
	Name string `json:"name" yaml:"name"`

	// Value of the label
	Value string `json:"value" yaml:"value"`
}

// Selector is used to match a set of resources given a label name and value
type Selector struct {
	// Name of the selector
	Name string `json:"name" yaml:"name"`

	// Value of the selector
	Value string `json:"value" yaml:"value"`
}

// Port is which ports should be forwarded to a service and it's corresponding
// resources
type Port struct {
	// Name denotes the puprose of the port
	Name string `json:"name" yaml:"name"`

	// Number is the port number
	Number int32 `json:"number" yaml:"number"`

	// Protocol is the protocol to use
	Protocol string `json:"protocol" yaml:"protocol"`
}

// ServiceConfiguration contains the service configuration
type ServiceConfiguration struct {
	// Name of the service
	Name string `json:"name" yaml:"name"`

	// Labels for the service
	Labels []Label `json:"labels" yaml:"labels"`

	// Selectors for the service
	Selectors []Selector `json:"selectors" yaml:"selectors"`

	// Ports for the service
	Ports []Port `json:"ports" yaml:"ports"`

	// ClusterIP specifies if a ClusterIP should be associated or not
	ClusterIP bool `json:"clusterIP" yaml:"clusterIP"`
}

// IsolationGroup defines the name of zone as well attributes for the zone configuration
// TODO(PS) Should this belong within the service configurations?
type IsolationGroup struct {
	// Name
	Name string `json:"name" yaml:"name"`

	// NumInstances defines the number of instances
	NumInstances int32 `json:"numInstances" yaml:"numInstances"`
}

// Resources defines CPU / Memory restrictions on pods
type Resources struct {
	Requests MemoryCPU `json:"requests" yaml:"requests"`
	Limits   MemoryCPU `json:"limits" yaml:"limits"`
}

// MemoryCPU defines memory cpu options
type MemoryCPU struct {
	// Memory defines max amount of memory
	Memory string `json:"memory" yaml:"memory"`

	// CPU defines max amount of CPU
	CPU string `json:"cpu" yaml:"cpu"`
}
