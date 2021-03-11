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
	"math"
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
	"go.uber.org/atomic"
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
	pods []*corev1.Pod,
	replicationFactor int,
	numInstances int32,
	onDeleteUpdateStrategy bool,
) (*myspec.M3DBCluster, *testDeps) {
	cfgMapName := defaultConfigMapName
	cluster := &myspec.M3DBCluster{
		ObjectMeta: clusterMeta,
		Spec: myspec.ClusterSpec{
			Image:                  defaultTestImage,
			ReplicationFactor:      int32(replicationFactor),
			ConfigMapName:          &cfgMapName,
			OnDeleteUpdateStrategy: onDeleteUpdateStrategy,
		},
	}
	cluster.ObjectMeta.UID = "abcd"
	groups := make([]myspec.IsolationGroup, 0, replicationFactor)
	for i := 0; i < replicationFactor; i++ {
		group := myspec.IsolationGroup{
			Name:         fmt.Sprintf("group%d", i),
			NumInstances: numInstances,
		}
		groups = append(groups, group)
	}
	cluster.Spec.IsolationGroups = groups
	cluster.ObjectMeta.Finalizers = []string{labels.EtcdDeletionFinalizer}

	objects := make([]runtime.Object, len(sets))
	statefulSets := make([]*appsv1.StatefulSet, len(sets))
	for i, meta := range sets {
		set, err := m3db.GenerateStatefulSet(cluster, groups[i].Name, numInstances)
		require.NoError(t, err)

		set.Namespace = meta.Namespace
		set.Status.ReadyReplicas = numInstances
		for k, v := range meta.Labels {
			set.Labels[k] = v
		}
		for k, v := range meta.Annotations {
			set.Annotations[k] = v
		}

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

	for _, pod := range pods {
		// nolint:makezero
		objects = append(objects, runtime.Object(pod))
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

type waitForStatefulSetsOptions struct {
	setReadyReplicas        bool
	expectedStatefulSets    []string
	simulatePodsNotUpdated  bool
	simulateStaleGeneration bool
	expectError             string
	currentRevision         string
}

type waitForStatefulSetsResult struct {
	updatedStatefulSets      []string
	failedUpdateStatefulSets []string
	podUpdateGroups          [][]string
}

func waitForStatefulSets(
	t *testing.T,
	controller *M3DBController,
	cluster *myspec.M3DBCluster,
	verb string,
	opts waitForStatefulSetsOptions,
) (waitForStatefulSetsResult, bool) {
	var (
		mu          sync.Mutex
		updates     int
		actualSets  = make(map[string]bool)
		updatedPods []string
	)

	controller.kubeClient.(*kubefake.Clientset).PrependReactor(verb, "statefulsets", func(action ktesting.Action) (bool, runtime.Object, error) {
		var sts *appsv1.StatefulSet
		switch verb {
		case "create":
			sts = action.(kubetesting.CreateActionImpl).GetObject().(*appsv1.StatefulSet)
		case "update":
			sts = action.(kubetesting.UpdateActionImpl).GetObject().(*appsv1.StatefulSet)
			// Note: Simulate an update revision to make sure the updated
			// replicas check is enforced.
			updates++
			sts.Status.UpdateRevision = fmt.Sprintf("updated-revision-%d", updates)
			sts.Status.CurrentRevision = fmt.Sprintf("%s-%s", sts.Name, opts.currentRevision)
			if !opts.simulatePodsNotUpdated {
				// Simulate all update immediately, unless explicitly
				// testing for when not updating them.
				sts.Status.UpdatedReplicas = *sts.Spec.Replicas
			}
			if opts.simulateStaleGeneration {
				sts.Status.ObservedGeneration = 2
				sts.ObjectMeta.Generation = 1
			}
		default:
			t.Errorf("verb %s is not supported", verb)
		}

		if opts.setReadyReplicas {
			// Note: should always have replicas set.
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

	controller.kubeClient.(*kubefake.Clientset).PrependReactor(
		"delete", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
			podName := action.(kubetesting.DeleteActionImpl).GetName()
			mu.Lock()
			updatedPods = append(updatedPods, podName)
			mu.Unlock()

			return false, nil, nil
		})

	// Iterate through the expected stateful sets twice (or at least 5 times) to make sure
	// we see all stateful sets that we expect and also be able to catch any extra stateful
	// sets that we don't.
	var (
		iters           = math.Max(float64(2*len(opts.expectedStatefulSets)), 5)
		finalErr        error
		podUpdateGroups [][]string
	)
	for i := 0; i < int(iters); i++ {
		err := controller.handleClusterUpdate(cluster)
		if opts.expectError != "" {
			finalErr = err
		} else {
			require.NoError(t, err)
		}

		mu.Lock()
		var seen int
		for _, found := range actualSets {
			if found {
				seen++
			}
		}
		if len(updatedPods) > 0 {
			podUpdateGroups = append(podUpdateGroups, updatedPods)
			updatedPods = nil
		}
		mu.Unlock()

		if seen != len(opts.expectedStatefulSets) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	var (
		updated []string
		failed  []string
		seen    int
	)
	// Start with all failed.
	for _, name := range opts.expectedStatefulSets {
		failed = append(failed, name)
	}
	for name, found := range actualSets {
		if found {
			seen++

			// Updated.
			updated = append(updated, name)

			// Remove from failed.
			filterFailed := failed[:]
			failed = failed[:0]
			for _, elem := range filterFailed {
				if elem != name {
					failed = append(failed, elem)
				}
			}
		}
	}

	if opts.expectError != "" {
		require.EqualError(t, finalErr, opts.expectError)
	}

	done := seen == len(opts.expectedStatefulSets)
	return waitForStatefulSetsResult{
		updatedStatefulSets:      updated,
		failedUpdateStatefulSets: failed,
		podUpdateGroups:          podUpdateGroups,
	}, done
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
	cluster, err := getClusterValue(pod)
	assert.Error(t, err)
	assert.Equal(t, "", cluster)

	pod.Labels = map[string]string{
		"operator.m3db.io/cluster": "foo",
	}

	cluster, err = getClusterValue(pod)
	assert.NoError(t, err)
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
			cluster, deps := setupTestCluster(
				t, *test.cluster, test.sets, nil, test.replicationFactor, 1, false,
			)
			defer deps.cleanup()
			c := deps.newController(t)

			_, done := waitForStatefulSets(t, c, cluster, "create", waitForStatefulSetsOptions{
				setReadyReplicas:     false,
				expectedStatefulSets: test.expCreateStatefulSets,
			})
			assert.True(t, done, "expected all sets to be created")
		})
	}
}

func TestHandleUpdateClusterUpdatesStatefulSets(t *testing.T) {
	tests := []struct {
		name                        string
		cluster                     *metav1.ObjectMeta
		sets                        []*metav1.ObjectMeta
		newImage                    string
		newConfigMap                string
		increaseReplicas            bool
		simulatePodsNotUpdated      bool
		simulateStaleGeneration     bool
		expUpdateStatefulSets       []string
		expFailedUpdateStatefulSets []string
		expError                    string
		expNotDone                  bool
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
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "enabled",
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
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "enabled",
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
					annotations.Update: "enabled",
				}),
			},
			newImage:              "m3db:v2.0.0",
			expUpdateStatefulSets: []string{"cluster1-rep2"},
		},
		{
			name: "removes update annotation even if stateful set doesn't change",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, nil),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep2", nil, nil),
			},
			expUpdateStatefulSets: []string{"cluster1-rep1"},
		},
		{
			name: "doesn't call update for replica changes",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "enabled",
				}),
			},
			increaseReplicas:      true,
			expUpdateStatefulSets: []string{},
		},
		{
			name: "updates stateful sets does not progress if pods not updated",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "enabled",
				}),
			},
			newImage:                    "m3db:v2.0.0",
			simulatePodsNotUpdated:      true,
			expUpdateStatefulSets:       []string{"cluster1-rep0", "cluster1-rep1", "cluster1-rep2"},
			expFailedUpdateStatefulSets: []string{"cluster1-rep1", "cluster1-rep2"},
			expNotDone:                  true,
		},
		{
			name: "fails if generation is stale",
			cluster: newMeta("cluster1", map[string]string{
				"foo":                      "bar",
				"operator.m3db.io/app":     "m3db",
				"operator.m3db.io/cluster": "cluster1",
			}, nil),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep1", nil, map[string]string{
					annotations.Update: "enabled",
				}),
				newMeta("cluster1-rep2", nil, map[string]string{
					annotations.Update: "enabled",
				}),
			},
			newImage:                "m3db:v2.0.0",
			simulateStaleGeneration: true,
			expNotDone:              true,
			expError:                "set cluster1-rep0 generation is not up to date (current: 1, observed: 2)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const replicas = 3
			cluster, deps := setupTestCluster(t, *test.cluster, test.sets, nil, replicas, 1, false)
			defer deps.cleanup()
			c := deps.newController(t)

			mockPlacement(deps, replicas)

			if test.newImage != "" {
				cluster.Spec.Image = test.newImage
			}
			if test.newConfigMap != "" {
				cluster.Spec.ConfigMapName = &test.newConfigMap
			}
			if test.increaseReplicas {
				cluster.Spec.ReplicationFactor = replicas + 1
				cluster.Spec.IsolationGroups = append(cluster.Spec.IsolationGroups, myspec.IsolationGroup{
					Name:         fmt.Sprintf("group%d", replicas),
					NumInstances: 1,
				})
			}

			result, done := waitForStatefulSets(t, c, cluster, "update", waitForStatefulSetsOptions{
				setReadyReplicas:        true,
				expectedStatefulSets:    test.expUpdateStatefulSets,
				simulatePodsNotUpdated:  test.simulatePodsNotUpdated,
				simulateStaleGeneration: test.simulateStaleGeneration,
				expectError:             test.expError,
			})
			if test.expNotDone {
				assert.False(t, done, "expected not all sets to be updated")
			} else {
				assert.True(t, done, "expected all sets to be updated")
			}
			if len(test.expFailedUpdateStatefulSets) > 0 {
				assert.Equal(t, test.expFailedUpdateStatefulSets, result.failedUpdateStatefulSets)
			}
		})
	}
}

func TestHandleUpdateClusterOnDeleteStrategy(t *testing.T) {
	var (
		replicas        int32 = 3
		currentRevision       = "current-revision"
		clusterName           = "cluster1"
		newImage              = "m3db:v2.0.0"
		baseLabels            = map[string]string{
			"operator.m3db.io/app":     "m3db",
			"operator.m3db.io/cluster": "cluster",
		}
		rawCluster = newMeta(clusterName, baseLabels, nil)
	)
	tests := []struct {
		name                  string
		sets                  []*metav1.ObjectMeta
		pods                  []*corev1.Pod
		expUpdateStatefulSets []string
		expPodUpdateGroups    [][]string
		updateVal             int
	}{
		{
			name:                  "update all nodes at once",
			sets:                  generateSets(clusterName, replicas, "3", true),
			pods:                  generatePods(clusterName, replicas, 3, currentRevision),
			expUpdateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1", "cluster1-rep2"},
			expPodUpdateGroups: [][]string{
				{"cluster1-rep0-0", "cluster1-rep0-1", "cluster1-rep0-2"},
				{"cluster1-rep1-0", "cluster1-rep1-1", "cluster1-rep1-2"},
				{"cluster1-rep2-0", "cluster1-rep2-1", "cluster1-rep2-2"},
			},
			updateVal: 3,
		},
		{
			name:                  "update some nodes, not evenly divisible",
			sets:                  generateSets(clusterName, replicas, "2", true),
			pods:                  generatePods(clusterName, replicas, 3, currentRevision),
			expUpdateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1", "cluster1-rep2"},
			expPodUpdateGroups: [][]string{
				{"cluster1-rep0-0", "cluster1-rep0-1"},
				{"cluster1-rep0-2"},
				{"cluster1-rep1-0", "cluster1-rep1-1"},
				{"cluster1-rep1-2"},
				{"cluster1-rep2-0", "cluster1-rep2-1"},
				{"cluster1-rep2-2"},
			},
			updateVal: 2,
		},
		{
			name:                  "update one at a time",
			sets:                  generateSets(clusterName, replicas, "1", true),
			pods:                  generatePods(clusterName, replicas, 2, currentRevision),
			expUpdateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1", "cluster1-rep2"},
			expPodUpdateGroups: [][]string{
				{"cluster1-rep0-0"},
				{"cluster1-rep0-1"},
				{"cluster1-rep1-0"},
				{"cluster1-rep1-1"},
				{"cluster1-rep2-0"},
				{"cluster1-rep2-1"},
			},
			updateVal: 1,
		},
		{
			name: "only update statefulset with annotation",
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil, map[string]string{
					annotations.ParallelUpdate: "1",
				}),
				newMeta("cluster1-rep1", nil, nil),
				newMeta("cluster1-rep2", nil, nil),
			},
			pods:                  generatePods(clusterName, replicas, 2, currentRevision),
			expUpdateStatefulSets: []string{"cluster1-rep0"},
			expPodUpdateGroups: [][]string{
				{"cluster1-rep0-0"},
				{"cluster1-rep0-1"},
			},
			updateVal: 1,
		},
		{
			name:                  "no update annotation, do nothing",
			sets:                  generateSets(clusterName, replicas, "3", false),
			pods:                  generatePods(clusterName, replicas, 3, currentRevision),
			expUpdateStatefulSets: []string{},
			updateVal:             3,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes := int32(len(test.pods))
			cluster, deps := setupTestCluster(
				t, *rawCluster, test.sets, test.pods, int(replicas), nodes, true,
			)
			defer deps.cleanup()
			c := deps.newController(t)

			mockPlacement(deps, replicas)

			cluster.Spec.Image = newImage

			result, done := waitForStatefulSets(t, c, cluster, "update", waitForStatefulSetsOptions{
				setReadyReplicas:     true,
				expectedStatefulSets: test.expUpdateStatefulSets,
				currentRevision:      currentRevision,
			})
			assert.True(t, done, "expected all sets to be updated")
			assert.Equal(t, test.expPodUpdateGroups, result.podUpdateGroups)
		})
	}
}

func TestHandleUpdateClusterFrozen(t *testing.T) {
	var (
		clusterMeta = newMeta("cluster1", map[string]string{
			"foo":                      "bar",
			"operator.m3db.io/app":     "m3db",
			"operator.m3db.io/cluster": "cluster1",
		}, nil)
		sets = []*metav1.ObjectMeta{
			newMeta("cluster1-rep0", nil, nil),
		}
	)
	cluster, deps := setupTestCluster(t, *clusterMeta, sets, nil, 3, 1, false)
	defer deps.cleanup()
	controller := deps.newController(t)

	cluster.Spec.Frozen = true

	count := atomic.NewInt64(0)
	controller.kubeClient.(*kubefake.Clientset).PrependReactor(
		"*", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
			count.Inc()
			return false, nil, nil
		})

	for i := 0; i < 20; i++ {
		err := controller.handleClusterUpdate(cluster)
		require.NoError(t, err)
	}

	assert.Equal(t, int64(0), count.Load())
}

func generateSets(
	// nolint:unparam
	clusterName string,
	rf int32,
	updateVal string,
	withAnnotation bool,
) []*metav1.ObjectMeta {
	var sets []*metav1.ObjectMeta
	for i := 0; i < int(rf); i++ {
		var ann map[string]string
		if withAnnotation {
			ann = map[string]string{
				annotations.ParallelUpdate: updateVal,
			}
		}
		set := newMeta(fmt.Sprintf("%s-rep%d", clusterName, i), nil, ann)
		sets = append(sets, set)
	}

	return sets
}

// nolint:unparam
func generatePods(clusterName string, rf int32, nodes int32, revision string) []*corev1.Pod {
	var pods []*corev1.Pod
	for i := 0; i < int(rf); i++ {
		for j := 0; j < int(nodes); j++ {
			pods = append(pods, &corev1.Pod{
				ObjectMeta: *newMeta(fmt.Sprintf("%s-rep%d-%d", clusterName, i, j), map[string]string{
					"controller-revision-hash": fmt.Sprintf("%s-rep%d-%s", clusterName, i, revision),
				}, nil),
			})
		}
	}

	return pods
}

func mockPlacement(deps *testDeps, replicas int32) {
	deps.namespaceClient.EXPECT().List().AnyTimes().Return(&admin.NamespaceGetResponse{
		Registry: &namespacepb.Registry{},
	}, nil)

	var mockInstances []placement.Instance
	for i := 0; i < int(replicas); i++ {
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
}
