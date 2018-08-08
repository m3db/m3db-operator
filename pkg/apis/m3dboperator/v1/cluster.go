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
	Status            CRDStatus   `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// M3DBClusterList represents a list of M3DB Clusters
type M3DBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []M3DBCluster `json:"items"`
}

type CRDStatus struct {
	State   CRDState `json:"state,omitempty"`
	Message string   `json:"message,omitempty"`
}

type CRDState string

// ClusterSpec defines cluster options
type ClusterSpec struct {
	// Image specifies which docker image to use with the cluster
	Image string `json:"image"`

	// ReplicationFactor defines how many replicas
	ReplicationFactor int32 `json:"replicationFactor"`

	// NumberOfShards defines how many shards in total
	NumberOfShards int32 `json:"numberOfShards"`

	// Zones specifies a map of key-value pairs. Defines which zones
	// to deploy persistent volumes for data nodes
	Zones []string `json:"zones,omitempty"`

	// DataDiskSize specifies how large the persistent volume should be attached
	// to the data nodes in the ES cluster
	DataDiskSize string `json:"data-volume-size"`

	// Resources defines memory / cpu constraints
	Resources Resources `json:"resources"`
}

// Resources defines CPU / Memory restrictions on pods
type Resources struct {
	Requests MemoryCPU `json:"requests"`
	Limits   MemoryCPU `json:"limits"`
}

// MemoryCPU defines memory cpu options
type MemoryCPU struct {
	// Memory defines max amount of memory
	Memory string `json:"memory"`

	// CPU defines max amount of CPU
	CPU string `json:"cpu"`
}
