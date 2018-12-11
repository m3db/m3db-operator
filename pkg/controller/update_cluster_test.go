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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"
	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	"github.com/m3db/m3/src/cluster/placement"
	"github.com/m3db/m3/src/cluster/shard"
	dbns "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type namespaceMatcher struct {
	name string
}

func (n namespaceMatcher) Matches(x interface{}) bool {
	return x.(*admin.NamespaceAddRequest).Name == n.name
}

func (namespaceMatcher) String() string {
	return "matches whether a namespaces name is expected"
}

func TestReconcileNamespaces(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	nsMock := deps.namespaceClient

	controller := deps.newController()
	defer deps.cleanup()

	registry := &dbns.Registry{
		Namespaces: map[string]*dbns.NamespaceOptions{
			"a": &dbns.NamespaceOptions{},
		},
	}
	resp := &admin.NamespaceGetResponse{
		Registry: registry,
	}
	nsMock.EXPECT().List().Return(resp, nil)

	nsMock.EXPECT().Delete("a").Return(nil)
	nsMock.EXPECT().Create(namespaceMatcher{"metrics-10s:2d"}).Return(nil)

	err := controller.reconcileNamespaces(cluster)
	assert.NoError(t, err)
}

func TestCleanupNamespaces(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	cluster.Spec.Namespaces = []myspec.Namespace{}

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	nsMock := deps.namespaceClient

	controller := deps.newController()
	defer deps.cleanup()

	registry := &dbns.Registry{Namespaces: map[string]*dbns.NamespaceOptions{
		"foo": &dbns.NamespaceOptions{},
	}}

	nsMock.EXPECT().Delete("foo").Return(nil)
	err := controller.pruneNamespaces(cluster, registry)
	assert.NoError(t, err)

	nsMock.EXPECT().Delete("foo").Return(m3admin.ErrNotFound)
	err = controller.pruneNamespaces(cluster, registry)
	assert.NoError(t, err)

	nsMock.EXPECT().Delete("foo").Return(errors.New("foo"))
	err = controller.pruneNamespaces(cluster, registry)
	assert.Error(t, err)

	registry.Namespaces["baz"] = &dbns.NamespaceOptions{}
	nsMock.EXPECT().Delete("foo").Return(nil)
	nsMock.EXPECT().Delete("baz").Return(nil)
	err = controller.pruneNamespaces(cluster, registry)
	assert.NoError(t, err)
}

func TestCreateNamespaces(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	cluster.Spec.Namespaces = append(cluster.Spec.Namespaces, myspec.Namespace{
		Name:   "foo",
		Preset: "10s:2d",
	})

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	nsMock := deps.namespaceClient

	controller := deps.newController()
	defer deps.cleanup()

	registry := &dbns.Registry{Namespaces: map[string]*dbns.NamespaceOptions{}}

	nsMock.EXPECT().Create(namespaceMatcher{"metrics-10s:2d"}).Return(nil)
	nsMock.EXPECT().Create(namespaceMatcher{"foo"}).Return(nil)

	err := controller.createNamespaces(cluster, registry)
	assert.NoError(t, err)
}

func TestNamespacesToCreate(t *testing.T) {
	tests := []struct {
		registry   *dbns.Registry
		namespaces []myspec.Namespace
		exp        []myspec.Namespace
	}{
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": &dbns.NamespaceOptions{},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "foo", Preset: "bar"},
			},
		},
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": &dbns.NamespaceOptions{},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "foo", Preset: "bar"},
				{Name: "baz", Preset: "qux"},
			},
			exp: []myspec.Namespace{
				{Name: "baz", Preset: "qux"},
			},
		},
	}

	for _, test := range tests {
		res := namespacesToCreate(test.registry, test.namespaces)
		assert.Equal(t, test.exp, res)
	}
}

func TestNamespacesToDelete(t *testing.T) {
	tests := []struct {
		registry   *dbns.Registry
		namespaces []myspec.Namespace
		exp        []string
	}{
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": &dbns.NamespaceOptions{},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "foo", Preset: "bar"},
			},
		},
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": &dbns.NamespaceOptions{},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "foo", Preset: "bar"},
				{Name: "baz", Preset: "qux"},
			},
		},
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": &dbns.NamespaceOptions{},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "baz", Preset: "qux"},
			},
			exp: []string{"foo"},
		},
	}

	for _, test := range tests {
		res := namespacesToDelete(test.registry, test.namespaces)
		assert.Equal(t, test.exp, res)
	}
}

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

	const cond = myspec.ClusterConditionPlacementInitialized
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
	cluster := getFixture("cluster-simple.yaml", t)

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	controller := deps.newController()
	defer deps.cleanup()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-a",
			Labels: map[string]string{
				"operator.m3db.io/isolation-group": "zone-a",
			},
		},
	}

	deps.idProvider.EXPECT().Identity(pod, cluster).Return(&myspec.PodIdentity{}, nil)

	expInstance := placementpb.Instance{
		Id:             "{}",
		IsolationGroup: "zone-a",
		Zone:           "embedded",
		Endpoint:       "pod-a.m3dbnode-cluster-simple:9000",
		Hostname:       "pod-a.m3dbnode-cluster-simple",
		Port:           9000,
		Weight:         100,
	}

	deps.placementClient.EXPECT().Add(expInstance)

	err := controller.addPodToPlacement(cluster, pod)
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
				UID:       types.UID(strconv.Itoa(i)),
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

func placementFromPods(t *testing.T, cluster *myspec.M3DBCluster, pods []*corev1.Pod, idProvider podidentity.Provider) placement.Placement {
	insts := make([]placement.Instance, len(pods))
	for i, pod := range pods {
		instPb, err := k8sops.PlacementInstanceFromPod(cluster, pod, idProvider)
		require.NoError(t, err)
		inst, err := placement.NewInstanceFromProto(instPb)
		require.NoError(t, err)
		insts[i] = inst
	}
	return placement.NewPlacement().SetInstances(insts)
}

func identityForPod(pod *corev1.Pod) *myspec.PodIdentity {
	return &myspec.PodIdentity{
		Name: pod.Name,
		UID:  string(pod.UID),
	}
}

type podNameMatcher struct {
	name string
}

func (p podNameMatcher) String() string {
	return "matches pods by name"
}

func (p podNameMatcher) Matches(x interface{}) bool {
	return x.(*corev1.Pod).Name == p.name
}

func newPodNameMatcher(name string) gomock.Matcher {
	return podNameMatcher{name: name}
}

func TestExpandPlacementForSet(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)
	set.Status.ReadyReplicas = 3

	pods := podsForClusterSet(cluster, set, 3)
	deps := newTestDeps(t, &testOpts{
		kubeObjects: append(objectsFromPods(pods...)),
		crdObjects:  []runtime.Object{cluster},
	})
	placementMock := deps.placementClient
	idProvider := deps.idProvider
	controller := deps.newController()
	defer deps.cleanup()

	for _, pod := range pods {
		pod := pod
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name), gomock.Any()).Return(identityForPod(pod), nil).AnyTimes()
	}

	pl := placementFromPods(t, cluster, pods[0:2], idProvider)
	group := cluster.Spec.IsolationGroups[0]

	instPb, err := k8sops.PlacementInstanceFromPod(cluster, pods[2], idProvider)
	require.NoError(t, err)

	placementMock.EXPECT().Add(*instPb)
	err = controller.expandPlacementForSet(cluster, set, group, pl)
	assert.NoError(t, err)

	cluster, err = deps.crdClient.Operator().M3DBClusters(cluster.Namespace).Get(cluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, cluster.Status.HasPodBootstrapping())
}

func TestExpandPlacementForSet_Nop(t *testing.T) {
	deps := newTestDeps(t, &testOpts{})
	controller := deps.newController()
	idProvider := deps.idProvider
	defer deps.cleanup()

	cluster := getFixture("cluster-3-zones.yaml", t)
	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 3)
	for _, pod := range pods {
		pod := pod
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name), gomock.Any()).Return(identityForPod(pod), nil).AnyTimes()
	}

	pl := placementFromPods(t, cluster, pods, idProvider)
	group := cluster.Spec.IsolationGroups[0]

	err = controller.expandPlacementForSet(cluster, set, group, pl)
	// We know this was a noop because the mock expects no calls.
	assert.NoError(t, err)
}

func TestExpandPlacementForSet_Err(t *testing.T) {
	deps := newTestDeps(t, &testOpts{})
	idProvider := deps.idProvider
	controller := deps.newController()
	defer deps.cleanup()

	cluster := getFixture("cluster-3-zones.yaml", t)
	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 2)
	group := cluster.Spec.IsolationGroups[0]
	for _, pod := range pods {
		pod := pod
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name), gomock.Any()).Return(identityForPod(pod), nil).AnyTimes()
	}

	pl := placementFromPods(t, cluster, pods, idProvider)
	const expErr = "cannot expand set 'cluster-zones-rep0', not yet ready"
	err = controller.expandPlacementForSet(cluster, set, group, pl)
	assert.Equal(t, expErr, err.Error())
}

func TestShrinkPlacementForSet(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 3)

	deps := newTestDeps(t, &testOpts{
		kubeObjects: objectsFromPods(pods...),
	})
	placementMock := deps.placementClient
	controller := deps.newController()
	defer deps.cleanup()

	deps.idProvider.EXPECT().Identity(newPodNameMatcher(pods[2].Name), cluster).Return(identityForPod(pods[2]), nil)
	placementMock.EXPECT().Remove(`{"name":"cluster-zones-rep0-2","uid":"2"}`)
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

func TestValidatePlacementWithStatus(t *testing.T) {

	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	placementMock := deps.placementClient
	defer deps.cleanup()

	controller := deps.newController()
	//idProvider := deps.idProvider

	placementMock.EXPECT().Get().AnyTimes()

	testBool, err := controller.validatePlacementWithStatus(cluster)

	require.NoError(t, err)
	require.True(t, testBool)

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

	var noPodIDs []*corev1.Pod
	noPods, err := sortPods(noPodIDs)
	require.Nil(t, noPods)
	require.Error(t, err)

}

func TestCheckPodsForReplacement(t *testing.T) {

	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	controller := deps.newController()
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	// normal pods in the placement
	podsForPlacement := podsForClusterSet(cluster, set, 3)

	// there should not be a replacement here
	noReplacePods := append(podsForPlacement, podsForPlacement[0])

	for _, pod := range noReplacePods {
		pod := pod
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name), gomock.Any()).Return(identityForPod(pod), nil).MaxTimes(2)
	}

	pl := placementFromPods(t, cluster, podsForPlacement, idProvider)

	testLeavingInstanceID, testNewPod, err := controller.checkPodsForReplacement(cluster, noReplacePods, pl)

	require.NoError(t, err)
	require.Contains(t, testLeavingInstanceID, "")
	require.Nil(t, testNewPod)

	// there should be a replace here
	replacePods := append(podsForPlacement, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podsForPlacement[1].Name,
			UID:  types.UID("321"),
			Labels: map[string]string{
				"different": "label",
			},
		}})

	idProvider.EXPECT().Identity(newPodNameMatcher(replacePods[len(replacePods)-1].Name),
		gomock.Any()).Return(identityForPod(replacePods[len(replacePods)-1]), nil).MaxTimes(5)

	testLeavingInstanceID, testNewPod, err = controller.checkPodsForReplacement(cluster, replacePods, pl)

	require.NoError(t, err)
	require.Contains(t, testLeavingInstanceID, podsForPlacement[1].Name)
	require.NotNil(t, testNewPod)

}

func TestReplacePodInPlacement(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	controller := deps.newController()
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	podsForPlacement := podsForClusterSet(cluster, set, 3)

	// this will be the new replacement pod, so it's not in the placement
	pods := append(podsForPlacement, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podsForPlacement[0].Name,
			UID:  types.UID("ABC"),
			Labels: map[string]string{
				"operator.m3db.io/isolation-group": "zone-a",
				"operator.m3db.io/cluster":         "cluster-zones",
			},
		},
	})

	for _, pod := range pods {
		pod := pod
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name), gomock.Any()).Return(identityForPod(pod), nil).MaxTimes(2)
	}

	pl := placementFromPods(t, cluster, podsForPlacement, idProvider)

	testLeavingInstanceID, testNewPod, err := controller.checkPodsForReplacement(cluster, pods, pl)

	require.NoError(t, err)
	require.Contains(t, testLeavingInstanceID, pods[0].Name)
	require.NotNil(t, testNewPod)

	expInstance := placementpb.Instance{
		Id:             "{\"name\":\"cluster-zones-rep0-0\",\"uid\":\"ABC\"}",
		IsolationGroup: "zone-a",
		Zone:           "embedded",
		Endpoint:       "cluster-zones-rep0-0.m3dbnode-cluster-zones:9000",
		Hostname:       "cluster-zones-rep0-0.m3dbnode-cluster-zones",
		Port:           9000,
		Weight:         100,
	}

	deps.placementClient.EXPECT().Replace(testLeavingInstanceID, expInstance)

	err = controller.replacePodInPlacement(cluster, pl, testLeavingInstanceID, testNewPod)
	require.NoError(t, err)

}

func TestReplacePodInPlacementWithError(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	controller := deps.newController()
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := k8sops.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	podsForPlacement := podsForClusterSet(cluster, set, 3)

	for _, pod := range podsForPlacement {
		pod := pod
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name), gomock.Any()).Return(identityForPod(pod), nil).MaxTimes(2)
	}

	pl := placementFromPods(t, cluster, podsForPlacement, idProvider)

	// error creating instance
	var badPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podsForPlacement[0].Name,
			UID:  types.UID("ABC"),
			Labels: map[string]string{
				"bad": "labels",
			},
		},
	}

	err = controller.replacePodInPlacement(cluster, pl, "dummy-id", badPod)
	require.Error(t, err)

	// error setting bootstrapping
	okPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"operator.m3db.io/isolation-group": "beep beep",
			},
		},
	}

	badCluster := getFixture("cluster-simple.yaml", t)

	idProvider.EXPECT().Identity(newPodNameMatcher(okPod.Name), gomock.Any()).Return(identityForPod(okPod), nil).MaxTimes(2)

	err = controller.replacePodInPlacement(badCluster, pl, "dummy-id", okPod)
	require.Error(t, err)
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
