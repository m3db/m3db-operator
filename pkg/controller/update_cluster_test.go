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
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	crdfake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
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
	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ktesting "k8s.io/client-go/testing"
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
	controller := deps.newController(t)
	defer deps.cleanup()

	registry := &dbns.Registry{
		Namespaces: map[string]*dbns.NamespaceOptions{
			"a": {},
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

func TestPruneNamespaces(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	cluster.Spec.Namespaces = []myspec.Namespace{}

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	nsMock := deps.namespaceClient
	controller := deps.newController(t)
	defer deps.cleanup()

	registry := &dbns.Registry{Namespaces: map[string]*dbns.NamespaceOptions{
		"foo": {},
	}}

	nsMock.EXPECT().Delete("foo").Return(nil)
	err := controller.pruneNamespaces(cluster, registry)
	assert.NoError(t, err)

	nsMock.EXPECT().Delete("foo").Return(pkgerrors.WithMessage(m3admin.ErrNotFound, "foo"))
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
	controller := deps.newController(t)
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
					"foo": {},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "foo", Preset: "bar"},
			},
		},
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": {},
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
					"foo": {},
				},
			},
			namespaces: []myspec.Namespace{
				{Name: "foo", Preset: "bar"},
			},
		},
		{
			registry: &dbns.Registry{
				Namespaces: map[string]*dbns.NamespaceOptions{
					"foo": {},
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
					"foo": {},
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
	controller := deps.newController(t)
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
	controller := deps.newController(t)
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
	controller := deps.newController(t)
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

	cluster, err := controller.reconcileBootstrappingStatus(cluster, pl)
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
	controller := deps.newController(t)
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

	cluster, err = controller.crdClient.OperatorV1alpha1().M3DBClusters(cluster.Namespace).Get(cluster.Name, metav1.GetOptions{})
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
		instPb, err := m3db.PlacementInstanceFromPod(cluster, pod, idProvider)
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
	uid  types.UID
}

func (p podNameMatcher) String() string {
	return fmt.Sprintf("matches pods by name == %s and UID == %s", p.name, p.uid)
}

func (p podNameMatcher) Matches(x interface{}) bool {
	pod := x.(*corev1.Pod)
	return pod.Name == p.name && pod.UID == p.uid
}

func newPodNameMatcher(name string, uid types.UID) gomock.Matcher {
	return podNameMatcher{
		name: name,
		uid:  uid,
	}
}

type identifyPodOptions struct {
	doErr bool
}

// identifyPods allows a mock ID provider to return a sane identify for a set of
// pods.
func identifyPods(idProvider *podidentity.MockProvider, pods []*corev1.Pod, opts *identifyPodOptions) {
	for _, pod := range pods {
		pod := pod
		id := identityForPod(pod)
		var err error
		if opts != nil && opts.doErr {
			id = nil
			err = errors.New("test")
		}
		idProvider.EXPECT().Identity(newPodNameMatcher(pod.Name, pod.UID), gomock.Any()).Return(id, err).AnyTimes()
	}
}

func TestExpandPlacementForSet(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)
	set.Status.ReadyReplicas = 3

	pods := podsForClusterSet(cluster, set, 3)
	deps := newTestDeps(t, &testOpts{
		kubeObjects: objectsFromPods(pods...),
		crdObjects:  []runtime.Object{cluster},
	})
	placementMock := deps.placementClient
	idProvider := deps.idProvider
	controller := deps.newController(t)
	defer deps.cleanup()

	identifyPods(idProvider, pods, nil)

	pl := placementFromPods(t, cluster, pods[:2], idProvider)
	group := cluster.Spec.IsolationGroups[0]

	instPb, err := m3db.PlacementInstanceFromPod(cluster, pods[2], idProvider)
	require.NoError(t, err)

	placementMock.EXPECT().Add(*instPb)
	err = controller.expandPlacementForSet(cluster, set, group, pl)
	assert.NoError(t, err)

	cluster, err = deps.crdClient.OperatorV1alpha1().M3DBClusters(cluster.Namespace).Get(cluster.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, cluster.Status.HasPodBootstrapping())
}

func TestExpandPlacementForSet_Nop(t *testing.T) {
	deps := newTestDeps(t, &testOpts{})
	controller := deps.newController(t)
	idProvider := deps.idProvider
	defer deps.cleanup()

	cluster := getFixture("cluster-3-zones.yaml", t)
	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 3)
	identifyPods(idProvider, pods, nil)

	pl := placementFromPods(t, cluster, pods, idProvider)
	group := cluster.Spec.IsolationGroups[0]

	err = controller.expandPlacementForSet(cluster, set, group, pl)
	// We know this was a noop because the mock expects no calls.
	assert.NoError(t, err)
}

func TestExpandPlacementForSet_Err(t *testing.T) {
	deps := newTestDeps(t, &testOpts{})
	idProvider := deps.idProvider
	controller := deps.newController(t)
	defer deps.cleanup()

	cluster := getFixture("cluster-3-zones.yaml", t)
	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 2)
	group := cluster.Spec.IsolationGroups[0]
	identifyPods(idProvider, pods, nil)

	pl := placementFromPods(t, cluster, pods, idProvider)
	const expErr = "cannot expand set 'cluster-zones-rep0', not yet ready"
	err = controller.expandPlacementForSet(cluster, set, group, pl)
	assert.Equal(t, expErr, err.Error())
}

func TestShrinkPlacementForSet(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)

	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	pods := podsForClusterSet(cluster, set, 3)
	deps := newTestDeps(t, &testOpts{
		kubeObjects: objectsFromPods(pods...),
	})
	placementMock := deps.placementClient
	controller := deps.newController(t)
	defer deps.cleanup()

	identifyPods(deps.idProvider, pods, nil)
	pl := placementFromPods(t, cluster, pods, deps.idProvider)

	// Expect the last pod to be removed.
	placementMock.EXPECT().Remove(`{"name":"cluster-zones-rep0-2","uid":"2"}`)
	err = controller.shrinkPlacementForSet(cluster, set, pl)
	assert.NoError(t, err)

	// If there are more pods in the set then in the placement, we expect the last
	// in the set to be removed.
	pl = placementFromPods(t, cluster, pods[:2], deps.idProvider)
	placementMock.EXPECT().Remove(`{"name":"cluster-zones-rep0-1","uid":"1"}`)
	err = controller.shrinkPlacementForSet(cluster, set, pl)
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

	controller := deps.newController(t)

	placementMock.EXPECT().Get().AnyTimes()

	clusterReturn, err := controller.validatePlacementWithStatus(cluster)

	require.NoError(t, err)
	require.NotNil(t, clusterReturn)
}

type placementInstancesMatcher struct {
	instanceNames []string
}

func (p placementInstancesMatcher) Matches(x interface{}) bool {
	pl := x.(*admin.PlacementInitRequest)
	plInsts := []string{}
	for _, inst := range pl.Instances {
		plInsts = append(plInsts, inst.Id)
	}

	sort.Strings(p.instanceNames)
	sort.Strings(plInsts)

	return reflect.DeepEqual(p.instanceNames, plInsts)
}

func (p placementInstancesMatcher) String() string {
	return fmt.Sprintf("matches whether instance names are equal to %v", p.instanceNames)
}

func TestValidatePlacementWithStatus_ErrNotFound(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)
	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)
	set.Status.ReadyReplicas = 3
	pods := podsForClusterSet(cluster, set, 3)

	deps := newTestDeps(t, &testOpts{
		kubeObjects: objectsFromPods(pods...),
		crdObjects:  []runtime.Object{cluster},
	})

	placementMock := deps.placementClient
	defer deps.cleanup()

	controller := deps.newController(t)

	idProvider := deps.idProvider
	expInsts := []string{
		`{"name":"cluster-zones-rep0-0","uid":"0"}`,
		`{"name":"cluster-zones-rep0-1","uid":"1"}`,
		`{"name":"cluster-zones-rep0-2","uid":"2"}`,
	}
	identifyPods(idProvider, pods, nil)
	matcher := placementInstancesMatcher{
		instanceNames: expInsts,
	}
	placementMock.EXPECT().Get().Return(nil, pkgerrors.Wrap(m3admin.ErrNotFound, "foo"))
	placementMock.EXPECT().Init(matcher)

	clusterReturn, err := controller.validatePlacementWithStatus(cluster)

	require.NoError(t, err)
	require.NotNil(t, clusterReturn)
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

func TestSortPods(t *testing.T) {
	for _, test := range []struct {
		name        string
		passNil     bool
		podNames    []string
		expPodNames []string
		expErr      bool
	}{
		{
			name:        "empty pods",
			expPodNames: []string{},
		},
		{
			name:    "nil pods",
			passNil: true,
			expErr:  true,
		},
		{
			name:     "bad pod name",
			podNames: []string{"foo"},
			expErr:   true,
		},
		{
			name:     "bad pod name w/ 1 valid",
			podNames: []string{"foo-1", "foo-bar"},
			expErr:   true,
		},
		{
			name:        "numerical pods",
			podNames:    []string{"foo-101", "foo-2"},
			expPodNames: []string{"foo-2", "foo-101"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pods := make([]*corev1.Pod, len(test.podNames))
			for i, name := range test.podNames {
				pods[i] = podWithName(name)
			}

			if test.passNil {
				pods = nil
			}
			podIDs, err := sortPods(pods)
			if test.expErr {
				assert.Error(t, err)
				return
			}

			podNames := make([]string, len(podIDs))
			for i, pod := range podIDs {
				podNames[i] = pod.pod.Name
			}

			assert.Equal(t, test.expPodNames, podNames)
		})
	}
}

func TestCheckPodsForReplacement(t *testing.T) {

	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	controller := deps.newController(t)
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	// normal pods in the placement
	podsForPlacement := podsForClusterSet(cluster, set, 3)

	// there should not be a replacement here
	identifyPods(idProvider, podsForPlacement, nil)

	pl := placementFromPods(t, cluster, podsForPlacement, idProvider)

	testLeavingInstanceID, testNewPod, err := controller.checkPodsForReplacement(cluster, podsForPlacement, pl)

	require.NoError(t, err)
	require.Equal(t, testLeavingInstanceID, "")
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

	identifyPods(idProvider, replacePods, nil)

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

	controller := deps.newController(t)
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
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

	identifyPods(idProvider, pods, nil)

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

	controller := deps.newController(t)
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)

	podsForPlacement := podsForClusterSet(cluster, set, 3)
	identifyPods(idProvider, podsForPlacement, nil)

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

	idProvider.EXPECT().Identity(newPodNameMatcher(okPod.Name, okPod.UID), gomock.Any()).Return(identityForPod(okPod), nil).MaxTimes(2)

	err = controller.replacePodInPlacement(badCluster, pl, "dummy-id", okPod)
	require.Error(t, err)
}

func TestFindPodInPlacement(t *testing.T) {
	var pl placement.Placement
	// This is hacky but test order matters: we construct a valid placement on the
	// first pass and keep a ref to it, such that on the test with a bad ID
	// provider we have a valid placement and can actually hit the paths we're
	// trying to test.
	for _, test := range []struct {
		name  string
		doErr bool
	}{
		{
			name: "valid ID",
		},
		{
			name:  "ID error",
			doErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cluster := getFixture("cluster-3-zones.yaml", t)
			deps := newTestDeps(t, &testOpts{
				crdObjects: []runtime.Object{cluster},
			})

			controller := deps.newController(t)
			idProvider := deps.idProvider
			defer deps.cleanup()

			set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
			require.NoError(t, err)
			pods := podsForClusterSet(cluster, set, 3)
			identifyPods(idProvider, pods, &identifyPodOptions{doErr: test.doErr})

			if !test.doErr {
				pl = placementFromPods(t, cluster, pods[:2], idProvider)
			}

			// We expect finding the first pod to return the first instance in the
			// placement, as they're both sorted.
			inst, err := controller.findPodInPlacement(cluster, pl, pods[0])
			if test.doErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, pl.Instances()[0], inst)

			// Looking for a pod not in the placement returns an error.
			inst, err = controller.findPodInPlacement(cluster, pl, pods[2])
			assert.Equal(t, errPodNotInPlacement, err)
			assert.Nil(t, inst)
		})
	}
}

func TestFindPodToRemove(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	controller := deps.newController(t)
	idProvider := deps.idProvider
	defer deps.cleanup()

	set, err := m3db.GenerateStatefulSet(cluster, "us-fake1-a", 3)
	require.NoError(t, err)
	pods := podsForClusterSet(cluster, set, 3)
	identifyPods(idProvider, pods, nil)

	pl := placementFromPods(t, cluster, pods, idProvider)

	inst, err := controller.findPodInPlacement(cluster, pl, pods[0])
	assert.NoError(t, err)
	assert.Equal(t, pl.Instances()[0], inst)

	// Can't remove from no pods.
	_, _, err = controller.findPodInstanceToRemove(cluster, pl, nil)
	assert.Equal(t, errEmptyPodList, err)

	// Can't remove from malformed pod names.
	_, _, err = controller.findPodInstanceToRemove(cluster, pl, []*corev1.Pod{
		podWithName("foo"),
	})
	assert.Contains(t, err.Error(), "cannot sort pods")

	// Removing from a placement w/ all pods removes the last.
	pod, inst, err := controller.findPodInstanceToRemove(cluster, pl, pods)
	assert.NoError(t, err)
	assert.Equal(t, pods[2], pod)
	assert.Equal(t, pl.Instances()[2], inst)

	// Removing from a placement w/ 2 insts and 3 pods removes the last pod that's
	// still in the placement.
	pl = placementFromPods(t, cluster, pods[:2], idProvider)
	pod, inst, err = controller.findPodInstanceToRemove(cluster, pl, pods)
	assert.NoError(t, err)
	assert.Equal(t, pods[1], pod)
	assert.Equal(t, pl.Instances()[1], inst)
}

func TestEtcdFinalizer(t *testing.T) {
	cluster := getFixture("cluster-3-zones.yaml", t)
	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})

	controller := deps.newController(t)
	defer deps.cleanup()

	// Mock the API to return errors so we know we don't hit in in
	// stringArrayContains checks.
	returnError := func() {
		controller.crdClient.(*crdfake.Clientset).PrependReactor("update", "m3dbclusters", func(action ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("test")
		})
	}

	reactorChain := []ktesting.Reactor{}
	for _, r := range controller.crdClient.(*crdfake.Clientset).Fake.ReactionChain {
		reactorChain = append(reactorChain, r)
	}
	// Helper to restore the reactor chain.
	noError := func() {
		controller.crdClient.(*crdfake.Clientset).Fake.ReactionChain = reactorChain
	}

	cluster2, err := controller.ensureEtcdFinalizer(cluster.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, stringArrayContains(cluster2.ObjectMeta.Finalizers, labels.EtcdDeletionFinalizer))

	// Flip the API to return errors so we know we don't hit Update() once there's
	// already a finalizer on the cluster.
	returnError()
	_, err = controller.ensureEtcdFinalizer(cluster2.DeepCopy())
	assert.NoError(t, err)
	_, err = controller.removeEtcdFinalizer(cluster2.DeepCopy())
	assert.EqualError(t, pkgerrors.Cause(err), "test")
	noError()

	cluster2, err = controller.removeEtcdFinalizer(cluster2.DeepCopy())
	assert.NoError(t, err)
	assert.Empty(t, cluster2.Finalizers)

	// API returns errors again and we know we don't hit it once the finalizer is
	// removed.
	returnError()
	_, err = controller.removeEtcdFinalizer(cluster2.DeepCopy())
	assert.NoError(t, err)
	noError()

	// Ensure we only remove the finalizer we care about.
	cluster2 = cluster.DeepCopy()
	cluster2.Finalizers = []string{"foo"}
	cluster2, err = controller.removeEtcdFinalizer(cluster2.DeepCopy())
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, cluster2.Finalizers)

	cluster2 = cluster.DeepCopy()
	cluster2.Finalizers = []string{"foo", labels.EtcdDeletionFinalizer}
	cluster2, err = controller.removeEtcdFinalizer(cluster2.DeepCopy())
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, cluster2.Finalizers)
}

func TestDeletePlacement(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{cluster},
	})
	controller := deps.newController(t)
	defer deps.cleanup()

	deps.placementClient.EXPECT().Get().Return(nil, errors.New("TEST"))
	err := controller.deletePlacement(cluster)
	assert.EqualError(t, pkgerrors.Cause(err), "TEST")

	deps.placementClient.EXPECT().Get().Return(nil, m3admin.ErrNotFound)
	err = controller.deletePlacement(cluster)
	assert.NoError(t, err)

	deps.placementClient.EXPECT().Get().Return(placement.NewPlacement(), nil)
	deps.placementClient.EXPECT().Delete().Return(errors.New("TEST2"))
	err = controller.deletePlacement(cluster)
	assert.EqualError(t, pkgerrors.Cause(err), "TEST2")

	deps.placementClient.EXPECT().Get().Return(placement.NewPlacement(), nil)
	deps.placementClient.EXPECT().Delete().Return(nil)
	err = controller.deletePlacement(cluster)
	assert.NoError(t, err)
}

func TestDeleteAllNamespaces(t *testing.T) {
	testResp := &admin.NamespaceGetResponse{
		Registry: &dbns.Registry{
			Namespaces: map[string]*dbns.NamespaceOptions{
				"ns1": nil,
				"ns2": nil,
			},
		},
	}

	// Need to run 2 tests so the mock recorder gets reset.
	t.Run("err", func(t *testing.T) {
		cluster := getFixture("cluster-simple.yaml", t)
		deps := newTestDeps(t, &testOpts{
			crdObjects: []runtime.Object{cluster},
		})
		controller := deps.newController(t)
		defer deps.cleanup()

		deps.namespaceClient.EXPECT().List().Return(nil, errors.New("TEST"))
		err := controller.deleteAllNamespaces(cluster)
		assert.EqualError(t, pkgerrors.Cause(err), "TEST")

		deps.namespaceClient.EXPECT().List().Return(&admin.NamespaceGetResponse{}, nil)
		err = controller.deleteAllNamespaces(cluster)
		assert.EqualError(t, pkgerrors.Cause(err), errNilNamespaceRegistry.Error())

		deps.namespaceClient.EXPECT().List().Return(testResp, nil)
		// Because of map iteration order, delete("ns2") may be called and stop
		// execution before delete("ns1") is called.
		deps.namespaceClient.EXPECT().Delete("ns1").AnyTimes().Return(nil)
		deps.namespaceClient.EXPECT().Delete("ns2").Return(errors.New("TEST"))
		err = controller.deleteAllNamespaces(cluster)
		assert.EqualError(t, pkgerrors.Cause(err), "TEST")
	})

	t.Run("success", func(t *testing.T) {
		cluster := getFixture("cluster-simple.yaml", t)
		deps := newTestDeps(t, &testOpts{
			crdObjects: []runtime.Object{cluster},
		})
		controller := deps.newController(t)
		defer deps.cleanup()

		deps.namespaceClient.EXPECT().List().Return(testResp, nil)
		deps.namespaceClient.EXPECT().Delete("ns1").Return(nil)
		deps.namespaceClient.EXPECT().Delete("ns2").Return(nil)
		err := controller.deleteAllNamespaces(cluster)
		assert.NoError(t, err)
	})
}

func TestStringArrayContains(t *testing.T) {
	tests := []struct {
		arr   []string
		s     string
		found bool
	}{
		{
			arr:   nil,
			s:     "foo",
			found: false,
		},
		{
			arr:   []string{},
			s:     "foo",
			found: false,
		},
		{
			arr:   []string{"foo", "bar"},
			s:     "foo",
			found: true,
		},
		{
			arr:   []string{"foo", "bar"},
			s:     "baz",
			found: false,
		},
	}

	for _, test := range tests {
		found := stringArrayContains(test.arr, test.s)
		assert.Equal(t, test.found, found, "expected to find %s in %v", test.s, test.arr)
	}
}
