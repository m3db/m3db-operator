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

package controller

import (
	"fmt"
	"sort"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	pkgplacement "github.com/m3db/m3db-operator/pkg/m3admin/placement"

	"github.com/m3db/m3cluster/generated/proto/placementpb"
	"github.com/m3db/m3cluster/placement"
	"github.com/m3db/m3cluster/shard"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetPodBootstrappingStatus(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	assert.False(t, cluster.Status.HasPodBootstrapping())

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	controller := deps.newController()
	defer deps.cleanup()

	cluster, err := controller.setStatusPodBootstrapping(cluster, corev1.ConditionTrue, "foo", "bar")
	assert.NoError(t, err)

	assert.True(t, cluster.Status.HasPodBootstrapping())
}

func TestSetStatus(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)

	fakeClock := clock.NewFakeClock(time.Now())
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
		clock:      fakeClock,
	})
	controller := deps.newController()
	defer deps.cleanup()

	const cond = myspec.ClusterConditionNamespaceInitialized
	cluster, err := controller.setStatus(cluster, cond, corev1.ConditionTrue, "foo", "bar")
	assert.NoError(t, err)

	c, ok := cluster.Status.GetCondition(cond)
	assert.True(t, ok)

	transitionTime := c.LastTransitionTime

	cluster, err = controller.setStatus(cluster, cond, corev1.ConditionTrue, "foo", "bar")
	assert.NoError(t, err)

	c, ok = cluster.Status.GetCondition(cond)
	assert.True(t, ok)
	assert.Equal(t, transitionTime, c.LastTransitionTime)

	fakeClock.Step(10 * time.Second)

	cluster, err = controller.setStatus(cluster, cond, corev1.ConditionFalse, "foo", "bar")
	assert.NoError(t, err)

	c, ok = cluster.Status.GetCondition(cond)
	assert.True(t, ok)
	assert.NotEqual(t, transitionTime, c.LastTransitionTime)
	assert.Equal(t, c.Status, corev1.ConditionFalse)
}

func TestReconcileBootstrappingStatus(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	controller := deps.newController()
	defer deps.cleanup()

	const cond = myspec.ClusterConditionPodBootstrapping

	newPl := func(state shard.State) placement.Placement {
		return placement.NewPlacement().SetInstances([]placement.Instance{
			placement.NewInstance().SetID("a").SetShards(shard.NewShards([]shard.Shard{
				shard.NewShard(1).SetState(state),
			})),
		})
	}

	pl := newPl(shard.Initializing)

	var err error
	cluster, err = controller.reconcileBootstrappingStatus(cluster, pl)
	assert.NoError(t, err)
	_, ok := cluster.Status.GetCondition(cond)
	assert.False(t, ok)

	pl = newPl(shard.Available)
	cluster, err = controller.reconcileBootstrappingStatus(cluster, pl)
	assert.NoError(t, err)
	c, ok := cluster.Status.GetCondition(cond)
	assert.True(t, ok)
	assert.Equal(t, cond, c.Type)
	assert.Equal(t, string(corev1.ConditionFalse), string(c.Status))
}

func TestAddPodToPlacement(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	cluster := getFixture("cluster-simple.yaml", t)

	placementMock := pkgplacement.NewMockClient(mc)
	deps := newTestDeps(t, &testOpts{
		crdObjects:      []runtime.Object{cluster},
		placementClient: placementMock,
	})
	controller := deps.newController()
	defer deps.cleanup()

	pl := placement.NewPlacement().SetReplicaFactor(1).SetMaxShardSetID(1)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-a",
			Labels: map[string]string{
				"operator.m3db.io/isolation-group": "zone-a",
			},
		},
	}

	expInstance := placementpb.Instance{
		Id:             "pod-a",
		IsolationGroup: "zone-a",
		Zone:           "embedded",
		Endpoint:       "pod-a.m3dbnode-cluster-simple:9000",
		Hostname:       "pod-a.m3dbnode-cluster-simple",
		Port:           9000,
		Weight:         100,
	}

	placementMock.EXPECT().Add(expInstance)

	err := controller.addPodToPlacement(cluster, pod, pl)
	assert.NoError(t, err)

	cluster, err = controller.crdClient.OperatorV1().M3DBClusters(cluster.Namespace).Get(cluster.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	assert.True(t, cluster.Status.HasPodBootstrapping())
}

func podsForClusterSet(cluster *myspec.M3DBCluster, set *appsv1.StatefulSet, numPods int) []*corev1.Pod {
	pods := make([]*corev1.Pod, numPods)
	for i := 0; i < numPods; i++ {
		podName := fmt.Sprintf("%s-%d", set.Name, i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: cluster.Namespace,
				Labels:    map[string]string{},
			},
		}
		for k, v := range set.Labels {
			pod.Labels[k] = v
		}
		pods[i] = pod
	}
	return pods
}

func objectsFromPods(pods ...*corev1.Pod) []runtime.Object {
	arr := make([]runtime.Object, len(pods))
	for i, pod := range pods {
		arr[i] = pod
	}
	return arr
}

func placementFromPods(t *testing.T, cluster *myspec.M3DBCluster, pods []*corev1.Pod) placement.Placement {
	insts := make([]placement.Instance, len(pods))
	for i, pod := range pods {
		instPb, err := k8sops.PlacementInstanceFromPod(cluster, pod)
		require.NoError(t, err)
		inst, err := placement.NewInstanceFromProto(instPb)
		require.NoError(t, err)
		insts[i] = inst
	}
	return placement.NewPlacement().SetInstances(insts)
}

func TestExpandPlacementForSet(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)
	set.Status.ReadyReplicas = 3

	pods := podsForClusterSet(cluster, set, 3)
	pl := placementFromPods(t, cluster, pods[0:2])
	group := cluster.Spec.IsolationGroups[0]

	placementMock := pkgplacement.NewMockClient(mc)
	deps := newTestDeps(t, &testOpts{
		kubeObjects:     append(objectsFromPods(pods...)),
		crdObjects:      []runtime.Object{cluster},
		placementClient: placementMock,
	})
	controller := deps.newController()
	defer deps.cleanup()

	instPb, err := k8sops.PlacementInstanceFromPod(cluster, pods[2])
	require.NoError(t, err)
	placementMock.EXPECT().Add(*instPb)

	err = controller.expandPlacementForSet(cluster, set, group, pl)
	assert.NoError(t, err)

	cluster, err = deps.crdClient.Operator().M3DBClusters(cluster.Namespace).Get(cluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, cluster.Status.HasPodBootstrapping())
}

func TestExpandPlacementForSet_Nop(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 3)
	pl := placementFromPods(t, cluster, pods)
	group := cluster.Spec.IsolationGroups[0]

	placementMock := pkgplacement.NewMockClient(mc)
	deps := newTestDeps(t, &testOpts{
		placementClient: placementMock,
	})
	controller := deps.newController()
	defer deps.cleanup()

	err = controller.expandPlacementForSet(cluster, set, group, pl)
	// We know this was a noop because the mock expects no calls.
	assert.NoError(t, err)
}

func TestExpandPlacementForSet_Err(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 2)
	pl := placementFromPods(t, cluster, pods)
	group := cluster.Spec.IsolationGroups[0]

	placementMock := pkgplacement.NewMockClient(mc)
	deps := newTestDeps(t, &testOpts{
		placementClient: placementMock,
	})
	controller := deps.newController()
	defer deps.cleanup()

	const expErr = "cannot expand set 'cluster-zones-rep0', not yet ready"
	err = controller.expandPlacementForSet(cluster, set, group, pl)
	assert.Equal(t, expErr, err.Error())
}

func TestShrinkPlacementForSet(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 3)

	placementMock := pkgplacement.NewMockClient(mc)
	deps := newTestDeps(t, &testOpts{
		kubeObjects:     objectsFromPods(pods...),
		placementClient: placementMock,
	})
	controller := deps.newController()
	defer deps.cleanup()

	placementMock.EXPECT().Remove(pods[2].Name)
	err = controller.shrinkPlacementForSet(cluster, set)
	assert.NoError(t, err)
}

func podWithName(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestSortPodID(t *testing.T) {
	for _, test := range []struct {
		podIDs []podID
		exp    []podID
	}{
		{
			podIDs: []podID{{nil, 1}, {nil, 2}},
			exp:    []podID{{nil, 1}, {nil, 2}},
		},
		{
			podIDs: []podID{{nil, 2}, {nil, 1}},
			exp:    []podID{{nil, 1}, {nil, 2}},
		},
	} {
		podIDs := test.podIDs
		sort.Sort(byPodID(podIDs))
		assert.Equal(t, test.exp, podIDs)
	}
}

func TestFindPodToRemove(t *testing.T) {
	pods := []*corev1.Pod{}

	_, err := findPodToRemove(pods)
	assert.Error(t, err)

	for _, s := range []string{
		"",
		"foo",
		"foo-bar",
	} {
		pods = []*corev1.Pod{podWithName(s)}
		_, err = findPodToRemove(pods)
		assert.Error(t, err)
	}

	tests := []struct {
		names  []string
		exp    *corev1.Pod
		expErr bool
	}{
		{
			names: []string{"foo-1", "foo-2", "foo-3"},
			exp:   podWithName("foo-3"),
		},
		{
			names: []string{"foo-100", "foo-2", "foo-1"},
			exp:   podWithName("foo-100"),
		},
		{
			names: []string{"foo-100", "foo-120", "foo-30"},
			exp:   podWithName("foo-120"),
		},
		{
			names:  []string{"foo-100", "foo-2", "foo-baz"},
			expErr: true,
		},
	}

	for _, test := range tests {
		pods := make([]*corev1.Pod, len(test.names))
		for i, name := range test.names {
			pods[i] = podWithName(name)
		}

		pod, err := findPodToRemove(pods)
		if test.expErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.exp, pod)
		}
	}
}
