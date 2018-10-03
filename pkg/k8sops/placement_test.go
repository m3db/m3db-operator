package k8sops

import (
	"testing"

	"github.com/m3db/m3cluster/generated/proto/placementpb"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func TestPlacementInstanceFromPod(t *testing.T) {
	cluster := &myspec.M3DBCluster{}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}

	_, err := PlacementInstanceFromPod(cluster, pod)
	assert.Error(t, err)

	cluster.ObjectMeta.Name = "cluster-a"
	pod.ObjectMeta.Labels[_labelIsolationGroup] = "zone-a"
	pod.ObjectMeta.Name = "pod-a"

	expInst := &placementpb.Instance{
		Id:             "pod-a",
		IsolationGroup: "zone-a",
		Zone:           "embedded",
		Weight:         100,
		Hostname:       "pod-a.m3dbnode-cluster-a",
		Endpoint:       "pod-a.m3dbnode-cluster-a:9000",
		Port:           9000,
	}

	inst, err := PlacementInstanceFromPod(cluster, pod)
	assert.NoError(t, err)
	assert.Equal(t, expInst, inst)
}
