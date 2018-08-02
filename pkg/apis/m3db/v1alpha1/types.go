package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterPhase string

const (
	ClusterPhaseInitial ClusterPhase = ""
	ClusterPhaseRunning              = "Running"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type M3DBServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []M3DBService `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type M3DBService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              M3DBSpec          `json:"spec"`
	Status            M3DBServiceStatus `json:"status,omitempty"`
}

type M3DBSpec struct {
	// Size is the size of the m3db deployment
	Size int32 `json:"size"`
}

type M3DBServiceStatus struct {
	// Nodes are the names of the m3db pods
	Phase ClusterPhase `json:"phase"`
}
