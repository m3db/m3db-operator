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
	"strconv"
	"sync"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	clientsetfake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	m3dbinformers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/k8sops/annotations"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	"github.com/m3db/m3/src/cluster/placement"
	namespacepb "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	kubetesting "k8s.io/client-go/testing"

	"github.com/golang/mock/gomock"
	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

const (
	defaultTestImage     = "m3db:v1.0.0"
	defaultConfigMapName = "configMapName"
)

func newMeta(name string, labels, annotations map[string]string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:        name,
		Labels:      labels,
		Annotations: annotations,
		Namespace:   "namespace",
	}
}

func setupTestCluster(
	t *testing.T,
	clusterMeta metav1.ObjectMeta,
	sets []*metav1.ObjectMeta,
	replicationFactor int,
) (*myspec.M3DBCluster, *testDeps) {
	cfgMapName := defaultConfigMapName
	cluster := &myspec.M3DBCluster{
		ObjectMeta: clusterMeta,
		Spec: myspec.ClusterSpec{
			Image:             defaultTestImage,
			ReplicationFactor: int32(replicationFactor),
			ConfigMapName:     &cfgMapName,
		},
	}
	cluster.ObjectMeta.UID = "abcd"
	for i := 0; i < replicationFactor; i++ {
		group := myspec.IsolationGroup{
			Name:         fmt.Sprintf("group%d", i),
			NumInstances: 1,
		}
		cluster.Spec.IsolationGroups = append(cluster.Spec.IsolationGroups, group)
	}
	cluster.ObjectMeta.Finalizers = []string{labels.EtcdDeletionFinalizer}

	objects := make([]runtime.Object, len(sets))
	statefulSets := make([]*appsv1.StatefulSet, len(sets))
	for i, s := range sets {
		set := &appsv1.StatefulSet{
			ObjectMeta: *s,
		}
		// Apply base labels
		if set.ObjectMeta.Labels == nil {
			set.ObjectMeta.Labels = make(map[string]string)
		}
		for k, v := range labels.BaseLabels(cluster) {
			set.ObjectMeta.Labels[k] = v
		}
		set.Spec.Template.Spec.Containers = append(set.Spec.Template.Spec.Containers, corev1.Container{
			Image: defaultTestImage,
		})
		set.Spec.Template.Spec.Volumes = append(set.Spec.Template.Spec.Volumes, corev1.Volume{
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: defaultConfigMapName},
				},
			},
		})
		statefulSets[i] = set
		objects[i] = set
		set.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(cluster, schema.GroupVersionKind{
				Group:   myspec.SchemeGroupVersion.Group,
				Version: myspec.SchemeGroupVersion.Version,
				Kind:    "m3dbcluster",
			}),
		}
	}

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{
			&myspec.M3DBCluster{
				ObjectMeta: clusterMeta,
			},
		},
		kubeObjects: objects,
	})

	return cluster, deps
}

func waitForStatefulSets(
	t *testing.T,
	controller *M3DBController,
	cluster *myspec.M3DBCluster,
	verb string,
	setReadyReplicas bool,
	expectedStatefulSets []string,
) bool {
	var (
		mu         sync.Mutex
		actualSets = make(map[string]bool)
	)

	controller.kubeClient.(*kubefake.Clientset).PrependReactor(verb, "statefulsets", func(action ktesting.Action) (bool, runtime.Object, error) {
		var sts *appsv1.StatefulSet
		switch verb {
		case "create":
			sts = action.(kubetesting.CreateActionImpl).GetObject().(*appsv1.StatefulSet)
		case "update":
			sts = action.(kubetesting.UpdateActionImpl).GetObject().(*appsv1.StatefulSet)
		default:
			t.Errorf("verb %s is not supported", verb)
		}

		if setReadyReplicas && sts.Spec.Replicas != nil {
			sts.Status.ReadyReplicas = *sts.Spec.Replicas
		}

		name := sts.Name
		mu.Lock()
		actualSets[name] = true
		mu.Unlock()

		// Return false to indicate that the next reactor in the chain should process this
		// object as well.
		return false, nil, nil
	})

	// Iterate through the expected stateful sets twice to make sure we see all stateful
	// sets that we expect and also be able to catch any extra stateful sets that we don't.
	var done bool
	for i := 0; i < 2*len(expectedStatefulSets); i++ {
		err := controller.handleClusterUpdate(cluster)
		require.NoError(t, err)

		mu.Lock()
		var seen int
		for _, found := range actualSets {
			if found {
				seen++
			}
		}
		mu.Unlock()

		if seen != len(expectedStatefulSets) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		done = true
	}
	return done
}

func TestNew(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	kubeClient := kubefake.NewSimpleClientset()
	crdClient := clientsetfake.NewSimpleClientset()
	client := m3db.NewMockK8sops(mc)
	idProvider := podidentity.NewMockProvider(mc)
	testOpts := []Option{
		WithScope(tally.NoopScope),
		WithLogger(zap.NewNop()),
		WithKClient(client),
		WithPodIdentityProvider(idProvider),
		WithCRDClient(crdClient),
		WithKubeClient(kubeClient),
		WithKubeInformerFactory(kubeinformers.NewSharedInformerFactory(kubeClient, 0)),
		WithFilteredInformerFactory(kubeinformers.NewSharedInformerFactory(kubeClient, 0)),
		WithM3DBClusterInformerFactory(m3dbinformers.NewSharedInformerFactory(crdClient, 0)),
	}

	controller, err := NewM3DBController(testOpts...)
	assert.NoError(t, err)
	assert.NotNil(t, controller)
}

func TestGetChildStatefulSets(t *testing.T) {
	tests := []struct {
		cluster     *metav1.ObjectMeta
		sets        []*metav1.ObjectMeta
		expChildren []string
	}{
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", nil, nil),
			},
			expChildren: []string{},
		},
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", map[string]string{
					"foo":                      "bar",
					"operator.m3db.io/app":     "m3db",
					"operator.m3db.io/cluster": "cluster1",
				}, nil),
			},
			expChildren: []string{"set1"},
		},
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", map[string]string{
					"foo":                      "bar",
					"operator.m3db.io/app":     "m3db",
					"operator.m3db.io/cluster": "cluster2",
				}, nil),
			},
			expChildren: []string{},
		},
	}

	for _, test := range tests {
		cluster := &myspec.M3DBCluster{
			ObjectMeta: *test.cluster,
		}
		cluster.ObjectMeta.UID = "abcd"

		objects := make([]runtime.Object, len(test.sets))
		statefulSets := make([]*appsv1.StatefulSet, len(test.sets))
		for i, s := range test.sets {
			set := &appsv1.StatefulSet{
				ObjectMeta: *s,
			}
			statefulSets[i] = set
			objects[i] = set
		}

		deps := newTestDeps(t, &testOpts{
			kubeObjects: objects,
		})

		c := deps.newController(t)

		// Ensure that with no owner references we don't act on these sets
		children, err := c.getChildStatefulSets(cluster)
		assert.NoError(t, err)
		assert.Empty(t, children)

		// Now set the owner refs and ensure we pick up the sets.
		for _, set := range statefulSets {
			set.SetOwnerReferences([]metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, schema.GroupVersionKind{
					Group:   myspec.SchemeGroupVersion.Group,
					Version: myspec.SchemeGroupVersion.Version,
					Kind:    "m3dbcluster",
				}),
			})
			_, err := deps.kubeClient.AppsV1().StatefulSets("namespace").Update(set)
			assert.NoError(t, err)
		}

		// Informer caches can take a bit to catch up
		var ok bool
		for i := 0; i < 5; i++ {
			children, err = c.getChildStatefulSets(cluster)
			assert.NoError(t, err)
			if len(children) != len(test.expChildren) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			ok = true
		}
		assert.True(t, ok, "expected to fetch sets")

		setNames := make([]string, len(test.expChildren))
		for i, child := range children {
			setNames[i] = child.Name
		}
		sort.Strings(setNames)
		assert.Equal(t, test.expChildren, setNames)

		deps.cleanup()
	}
}

func instanceWithGroup(id, group string) placement.Instance {
	return placement.NewInstance().
		SetID(id).
		SetIsolationGroup(group)
}

func TestInstancesInIsoGroup(t *testing.T) {
	i1 := instanceWithGroup("i1", "g1")
	i2 := instanceWithGroup("i2", "g1")
	i3 := instanceWithGroup("i3", "g2")

	pl := placement.NewPlacement().SetInstances([]placement.Instance{
		i1,
		i2,
		i3,
	})

	insts := instancesInIsoGroup(pl, "foo")
	assert.Empty(t, insts)

	insts = instancesInIsoGroup(pl, "g1")
	exp := []placement.Instance{i1, i2}
	assert.Equal(t, exp, insts)

	insts = instancesInIsoGroup(pl, "g2")
	exp = []placement.Instance{i3}
	assert.Equal(t, exp, insts)
}

func TestGetClusterValue(t *testing.T) {
	pod := &corev1.Pod{}
	cluster, ok := getClusterValue(pod)
	assert.False(t, ok)
	assert.Equal(t, "", cluster)

	pod.Labels = map[string]string{
		"operator.m3db.io/cluster": "foo",
	}

	cluster, ok = getClusterValue(pod)
	assert.True(t, ok)
	assert.Equal(t, "foo", cluster)
}

func TestGetParentCluster(t *testing.T) {
	cluster := &myspec.M3DBCluster{
		ObjectMeta: newObjectMeta("foo", nil),
	}

	deps := newTestDeps(t, &testOpts{
		crdObjects: []runtime.Object{
			cluster,
		},
	})
	c := deps.newController(t)
	defer deps.cleanup()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
		},
	}
	_, err := c.getParentCluster(pod)
	assert.Equal(t, errOrphanedPod, err)

	pod.Labels = map[string]string{
		"operator.m3db.io/cluster": "foo",
	}

	parentCluster, err := c.getParentCluster(pod)
	assert.NoError(t, err)
	assert.Equal(t, cluster, parentCluster)
}

func TestHandlePodUpdate(t *testing.T) {
	cluster := &myspec.M3DBCluster{
		ObjectMeta: newObjectMeta("foo", nil),
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "namespace",
			Labels: map[string]string{
				"operator.m3db.io/cluster": "foo",
			},
		},
	}

	deps := newTestDeps(t, &testOpts{
		crdObjects:  []runtime.Object{cluster},
		kubeObjects: []runtime.Object{pod1},
	})
	c := deps.newController(t)
	defer deps.cleanup()

	mockID := &myspec.PodIdentity{
		Name: "pod1",
		UID:  "foo",
	}

	deps.idProvider.EXPECT().Identity(pod1, cluster).Return(mockID, nil)

	err := c.handlePodUpdate(pod1)
	require.NoError(t, err)

	newPod, err := deps.kubeClient.CoreV1().Pods("namespace").Get("pod1", metav1.GetOptions{})
	assert.NoError(t, err)

	annotatedID, ok := newPod.Annotations["operator.m3db.io/pod-identity"]
	require.True(t, ok, "new pod must have annotated ID")
	expID := `{"name":"pod1","uid":"foo"}`
	assert.Equal(t, expID, annotatedID)
}

func TestClusterEventLoop(t *testing.T) {
	deps := newTestDeps(t, &testOpts{})
	defer deps.cleanup()

	c := deps.newController(t)

	cluster := &myspec.M3DBCluster{
		ObjectMeta: newObjectMeta("foo", nil),
	}
	c.enqueueCluster(cluster)

	wait.Poll(time.Millisecond, 5*time.Second, func() (bool, error) {
		return c.clusterWorkQueue.Len() == 1, nil
	})

	doneC := make(chan struct{})
	go func() {
		c.runClusterLoop()
		doneC <- struct{}{}
	}()

	wait.Poll(time.Millisecond, 5*time.Second, func() (bool, error) {
		return c.clusterWorkQueue.Len() == 0, nil
	})

	c.clusterWorkQueue.ShutDown()

	select {
	case <-time.After(5 * time.Second):
		t.Error("expected loop to finish within 10s")
	case <-doneC:
	}
}

func TestPodEventLoop(t *testing.T) {
	deps := newTestDeps(t, &testOpts{})
	defer deps.cleanup()

	c := deps.newController(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "namespace",
		},
	}
	c.enqueuePod(pod)

	wait.Poll(time.Millisecond, 5*time.Second, func() (bool, error) {
		return c.podWorkQueue.Len() == 1, nil
	})

	doneC := make(chan struct{})
	go func() {
		c.runPodLoop()
		doneC <- struct{}{}
	}()

	wait.Poll(time.Millisecond, 5*time.Second, func() (bool, error) {
		return c.podWorkQueue.Len() == 0, nil
	})

	c.podWorkQueue.ShutDown()

	select {
	case <-time.After(5 * time.Second):
		t.Error("expected loop to finish within 10s")
	case <-doneC:
	}
}

func TestValidateIsolationGroups(t *testing.T) {
	tests := []struct {
		groups   []myspec.IsolationGroup
		rf       int32
		doExpErr bool
		expErr   error
	}{
		{
			rf: 2,
			groups: []myspec.IsolationGroup{
				{Name: "foo"},
				{Name: "bar"},
			},
		},
		{
			rf: 3,
			groups: []myspec.IsolationGroup{
				{Name: "foo"},
				{Name: "bar"},
			},
			expErr:   errInvalidNumIsoGroups,
			doExpErr: true,
		},
		{
			rf: 2,
			groups: []myspec.IsolationGroup{
				{Name: "foo"},
				{Name: "foo"},
			},
			expErr:   errNonUniqueIsoGroups,
			doExpErr: true,
		},
	}

	for _, test := range tests {
		cluster := newM3DBCluster("ns", "cluster")
		cluster.Spec.ReplicationFactor = test.rf
		cluster.Spec.IsolationGroups = test.groups
		err := validateIsolationGroups(cluster)
		if test.doExpErr {
			assert.Error(t, err)
			assert.Equal(t, test.expErr, pkgerrors.Cause(err))
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestHandleUpdateClusterCreatesStatefulSets(t *testing.T) {
	tests := []struct {
		name                  string
		cluster               *metav1.ObjectMeta
		sets                  []*metav1.ObjectMeta
		replicationFactor     int
		expCreateStatefulSets []string
	}{
		{
			name: "creates missing stateful sets at tail",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, nil),
			},
			replicationFactor:     3,
			expCreateStatefulSets: []string{"cluster1-rep1", "cluster1-rep2"},
		},
		{
			name: "creates missing stateful sets at head and tail",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep1", nil, nil),
			},
			replicationFactor:     3,
			expCreateStatefulSets: []string{"cluster1-rep0", "cluster1-rep2"},
		},
		{
			name: "creates missing stateful sets at head",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep2", nil, nil),
			},
			replicationFactor:     3,
			expCreateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster, deps := setupTestCluster(t, *test.cluster, test.sets, test.replicationFactor)
			defer deps.cleanup()
			c := deps.newController(t)

			done := waitForStatefulSets(t, c, cluster, "create", false, test.expCreateStatefulSets)
			assert.True(t, done, "expected all sets to be created")
		})
	}
}

func TestHandleUpdateClusterUpdatesStatefulSets(t *testing.T) {
	tests := []struct {
		name                  string
		cluster               *metav1.ObjectMeta
		sets                  []*metav1.ObjectMeta
		newImage              string
		newConfigMap          string
		expUpdateStatefulSets []string
	}{
		{
			name: "updates stateful sets when image is out of date",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, map[string]string{
					annotations.Update: "true",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "true",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "true",
				}),
			},
			newImage:              "m3db:v2.0.0",
			expUpdateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1", "cluster1-rep2"},
		},
		{
			name: "updates stateful sets when config map name is out of date",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, map[string]string{
					annotations.Update: "true",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "true",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "true",
				}),
			},
			newConfigMap:          "configMapName-2",
			expUpdateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1", "cluster1-rep2"},
		},
		{
			name: "only updates stateful sets with update annotation",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, nil),
				newMeta("cluster1-rep1", nil, nil),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "true",
				}),
			},
			newImage:              "m3db:v2.0.0",
			expUpdateStatefulSets: []string{"cluster1-rep2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const replicas = 3
			cluster, deps := setupTestCluster(t, *test.cluster, test.sets, replicas)
			defer deps.cleanup()
			c := deps.newController(t)

			deps.namespaceClient.EXPECT().List().AnyTimes().Return(&admin.NamespaceGetResponse{
				Registry: &namespacepb.Registry{},
			}, nil)

			var mockInstances []placement.Instance
			for i := 0; i < replicas; i++ {
				inst := placement.NewMockInstance(deps.mockController)

				// Since the controller checks all instances are available in the placement after
				// checking and performing any updates, and we're only concerned with the latter
				// in this test, configure the mock instances to be unavailable to reduce the
				// amount of mocks we need to set up.
				inst.EXPECT().IsAvailable().AnyTimes().Return(false)
				inst.EXPECT().ID().AnyTimes().Return(strconv.Itoa(i))
				mockInstances = append(mockInstances, inst)
			}

			mockPlacement := placement.NewMockPlacement(deps.mockController)
			mockPlacement.EXPECT().NumInstances().AnyTimes().Return(len(mockInstances))
			mockPlacement.EXPECT().Instances().AnyTimes().Return(mockInstances)
			deps.placementClient.EXPECT().Get().AnyTimes().Return(mockPlacement, nil)

			if test.newImage != "" {
				cluster.Spec.Image = test.newImage
			}
			if test.newConfigMap != "" {
				cluster.Spec.ConfigMapName = &test.newConfigMap
			}

			done := waitForStatefulSets(t, c, cluster, "update", true, test.expUpdateStatefulSets)
			assert.True(t, done, "expected all sets to be updated")
		})
	}
}
