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
	"sort"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"github.com/m3db/m3cluster/placement"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stretchr/testify/assert"
)

func newMeta(name string, labels map[string]string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      name,
		Labels:    labels,
		Namespace: "namespace",
	}
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
				newMeta("set1", map[string]string{"foo": "bar"}),
			},
			expChildren: []string{"set1"},
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

		c := deps.newController()

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
