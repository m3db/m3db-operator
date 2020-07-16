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
	"sync"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	clientsetfake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	m3dbinformers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	"github.com/m3db/m3/src/cluster/placement"

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

func newMeta(name string, labels map[string]string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      name,
		Labels:    labels,
		Namespace: "namespace",
	}
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
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", nil),
			},
			expChildren: []string{},
		},
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", map[string]string{
					"foo":                      "bar",
					"operator.m3db.io/app":     "m3db",
					"operator.m3db.io/cluster": "cluster1",
				}),
			},
			expChildren: []string{"set1"},
		},
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", map[string]string{
					"foo":                      "bar",
					"operator.m3db.io/app":     "m3db",
					"operator.m3db.io/cluster": "cluster2",
				}),
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
			}),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep0", nil),
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
			}),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep1", nil),
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
			}),
			sets: []*metav1.ObjectMeta{
				newMeta("cluster1-rep2", nil),
			},
			replicationFactor:     3,
			expCreateStatefulSets: []string{"cluster1-rep0", "cluster1-rep1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfgMapName := "configMapName"
			cluster := &myspec.M3DBCluster{
				ObjectMeta: *test.cluster,
				Spec: myspec.ClusterSpec{
					ReplicationFactor: int32(test.replicationFactor),
					ConfigMapName:     &cfgMapName,
				},
			}
			cluster.ObjectMeta.UID = "abcd"
			for i := 0; i < test.replicationFactor; i++ {
				group := myspec.IsolationGroup{
					Name:         fmt.Sprintf("group%d", i),
					NumInstances: 1,
				}
				cluster.Spec.IsolationGroups = append(cluster.Spec.IsolationGroups, group)
			}
			cluster.ObjectMeta.Finalizers = []string{labels.EtcdDeletionFinalizer}

			objects := make([]runtime.Object, len(test.sets))
			statefulSets := make([]*appsv1.StatefulSet, len(test.sets))
			for i, s := range test.sets {
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
						ObjectMeta: *test.cluster,
					},
				},
				kubeObjects: objects,
			})
			defer deps.cleanup()
			c := deps.newController(t)

			// Keep running cluster updates until no more stateful sets required
			var expectedMu sync.Mutex
			expectedSetsCreated := make(map[string]bool)
			for _, name := range test.expCreateStatefulSets {
				expectedSetsCreated[name] = false
			}

			c.kubeClient.(*kubefake.Clientset).PrependReactor("create", "statefulsets", func(action ktesting.Action) (bool, runtime.Object, error) {
				cluster := action.(kubetesting.CreateActionImpl).GetObject().(*appsv1.StatefulSet)
				name := cluster.Name
				expectedMu.Lock()
				expectedSetsCreated[name] = true
				expectedMu.Unlock()
				return true, cluster, nil
			})

			var done bool
			for i := 0; i < 5; i++ {
				err := c.handleClusterUpdate(cluster)
				require.NoError(t, err)

				expectedMu.Lock()
				created := len(expectedSetsCreated)
				expectedMu.Unlock()
				if created != len(test.expCreateStatefulSets) {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				done = true
			}
			assert.True(t, done, "expected all sets to be created")
		})
	}
}
