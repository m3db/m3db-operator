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

package v1

import (
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	for _, test := range []struct {
		cond ClusterConditionType
		f    func(s *M3DBStatus) bool
	}{
		{
			cond: ClusterConditionNamespaceInitialized,
			f:    func(s *M3DBStatus) bool { return s.HasInitializedNamespace() },
		},
		{
			cond: ClusterConditionPlacementInitialized,
			f:    func(s *M3DBStatus) bool { return s.HasInitializedPlacement() },
		},
		{
			cond: ClusterConditionPodBootstrapping,
			f:    func(s *M3DBStatus) bool { return s.HasPodBootstrapping() },
		},
	} {
		t.Run(string(test.cond), func(t *testing.T) {
			status := &M3DBStatus{}
			assert.False(t, test.f(status))

			status.Conditions = []ClusterCondition{
				{
					Type:   test.cond,
					Status: corev1.ConditionFalse,
				},
			}

			assert.False(t, test.f(status))
			status.Conditions[0].Status = corev1.ConditionTrue
			assert.True(t, test.f(status))
		})
	}
}

func TestGetCondition(t *testing.T) {
	status := &M3DBStatus{}

	_, ok := status.GetCondition(ClusterConditionNamespaceInitialized)
	assert.False(t, ok)

	cond := ClusterCondition{
		Type:   ClusterConditionNamespaceInitialized,
		Reason: "foo",
	}

	status.UpdateCondition(cond)

	cond2, ok := status.GetCondition(ClusterConditionNamespaceInitialized)
	assert.True(t, ok)
	assert.Equal(t, cond, cond2)
}

func TestUpdateCondition(t *testing.T) {
	status := &M3DBStatus{}

	status.UpdateCondition(ClusterCondition{
		Type:   ClusterConditionNamespaceInitialized,
		Status: corev1.ConditionUnknown,
	})

	exp := &M3DBStatus{
		Conditions: []ClusterCondition{
			{
				Type:   ClusterConditionNamespaceInitialized,
				Status: corev1.ConditionUnknown,
			},
		},
	}

	assert.Equal(t, exp, status)

	status.UpdateCondition(ClusterCondition{
		Type:   ClusterConditionNamespaceInitialized,
		Status: corev1.ConditionTrue,
	})

	exp = &M3DBStatus{
		Conditions: []ClusterCondition{
			{
				Type:   ClusterConditionNamespaceInitialized,
				Status: corev1.ConditionTrue,
			},
		},
	}

	assert.Equal(t, exp, status)
}

func TestSortIsoGroups(t *testing.T) {
	groups := IsolationGroups([]IsolationGroup{
		{
			Name:         "b",
			NumInstances: 1,
		},
		{
			Name:         "a",
			NumInstances: 2,
		},
	})

	assert.False(t, sort.IsSorted(groups))

	sort.Sort(groups)

	expGroups := IsolationGroups([]IsolationGroup{
		{
			Name:         "a",
			NumInstances: 2,
		},
		{
			Name:         "b",
			NumInstances: 1,
		},
	})
	assert.Equal(t, expGroups, groups)
}
