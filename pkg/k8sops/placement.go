package k8sops

import (
	"fmt"

	"github.com/m3db/m3cluster/generated/proto/placementpb"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	corev1 "k8s.io/api/core/v1"
)

// PlacementInstanceFromPod creates a new m3cluster placement instance given a
// pod spec.
func PlacementInstanceFromPod(cluster *myspec.M3DBCluster, pod *corev1.Pod) (*placementpb.Instance, error) {
	isoGroup, ok := pod.ObjectMeta.Labels[_labelIsolationGroup]
	if !ok {
		return nil, fmt.Errorf("could not find label %s in %v", _labelIsolationGroup, pod.ObjectMeta.Labels)
	}

	setService := HeadlessServiceName(cluster.Name)
	hostname := pod.Name + "." + setService

	// TODO(schallert): dynamic zone, portname consts
	instance := &placementpb.Instance{
		Id:             pod.Name,
		IsolationGroup: isoGroup,
		Zone:           "embedded",
		Weight:         100,
		Hostname:       hostname,
		Endpoint:       fmt.Sprintf("%s:%d", hostname, 9000),
		Port:           9000,
	}

	return instance, nil
}
