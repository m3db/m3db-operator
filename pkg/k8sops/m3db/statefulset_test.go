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

package m3db

import (
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	"github.com/m3db/m3db-operator/pkg/k8sops/labels"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatefulSet(t *testing.T) {
	fixture := getFixture("testM3DBCluster.yaml", t)
	k := newFakeK8sops(t)
	require.NotNil(t, k)

	ssName := StatefulSetName(fixture.GetName(), 0)
	require.Equal(t, "m3db-cluster-rep0", ssName)

	require.Equal(t, "m3db-account1", fixture.Spec.ServiceAccountName)
}

func TestGenerateDownwardAPIVolume(t *testing.T) {
	vf := generateDownwardAPIVolume()
	exp := corev1.Volume{
		Name: "pod-identity",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "identity",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.annotations['operator.m3db.io/pod-identity']",
						},
					},
				},
			},
		},
	}
	assert.Equal(t, exp, vf)
}

func TestGenerateDownwardAPIVolumePath(t *testing.T) {
	vm := generateDownwardAPIVolumeMount()
	exp := corev1.VolumeMount{
		Name:      "pod-identity",
		MountPath: "/etc/m3db/pod-identity",
		ReadOnly:  false,
	}

	assert.Equal(t, exp, vm)
}

func TestGenerateStatefulSetNodeAffinity(t *testing.T) {
	type expTerm struct {
		key    string
		values []string
	}
	tests := []struct {
		isoGroup myspec.IsolationGroup
		expErr   error
		expTerms []expTerm
	}{
		{
			isoGroup: myspec.IsolationGroup{
				Name: "group1",
			},
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name: "group2",
				NodeAffinityTerms: []myspec.NodeAffinityTerm{
					{
						Key:    "foobar",
						Values: []string{"group2"},
					},
				},
			},
			expTerms: []expTerm{
				{
					key:    "foobar",
					values: []string{"group2"},
				},
			},
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name: "zone-and-inst-type",
				NodeAffinityTerms: []myspec.NodeAffinityTerm{
					{
						Key:    "zone",
						Values: []string{"zone-a"},
					},
					{
						Key:    "instance-type",
						Values: []string{"large"},
					},
				},
			},
			expTerms: []expTerm{
				{
					key:    "zone",
					values: []string{"zone-a"},
				},
				{
					key:    "instance-type",
					values: []string{"large"},
				},
			},
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name: "group3",
				NodeAffinityTerms: []myspec.NodeAffinityTerm{
					{
						Key: "foobar",
					},
				},
			},
			expErr: errEmptyNodeAffinityValues,
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name: "group4",
				NodeAffinityTerms: []myspec.NodeAffinityTerm{
					{
						Values: []string{"group2"},
					},
				},
			},
			expErr: errEmptyNodeAffinityKey,
		},
	}

	for _, test := range tests {
		nodeaffinity, err := GenerateStatefulSetNodeAffinity(test.isoGroup)
		if test.expErr != nil {
			assert.Equal(t, test.expErr, err)
			continue
		}

		if len(test.expTerms) == 0 {
			assert.Nil(t, nodeaffinity)
			continue
		}

		terms := nodeaffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		assert.Len(t, terms, 1)

		expTerms := make([]corev1.NodeSelectorRequirement, len(test.expTerms))
		for i, term := range test.expTerms {
			expTerms[i] = corev1.NodeSelectorRequirement{
				Key:      term.key,
				Operator: corev1.NodeSelectorOpIn,
				Values:   term.values,
			}
		}

		assert.Equal(t, expTerms, terms[0].MatchExpressions)
	}
}

func TestGenerateStatefulSetPodAntiAffinity(t *testing.T) {
	tests := []struct {
		isoGroup myspec.IsolationGroup
		expTerm  string
		expErr   error
	}{
		{
			isoGroup: myspec.IsolationGroup{
				Name: "group1",
			},
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name:               "group2",
				UsePodAntiAffinity: false,
			},
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name:                  "group3",
				UsePodAntiAffinity:    true,
				PodAffinityToplogyKey: "hostname",
			},
			expTerm: "hostname",
		},
		{
			isoGroup: myspec.IsolationGroup{
				Name:               "group4",
				UsePodAntiAffinity: true,
			},
			expErr: errEmptyPodAffinityToplogyKey,
		},
	}

	for _, test := range tests {
		antiaffinity, err := GenerateStatefulSetPodAntiAffinity(test.isoGroup)

		if test.expErr != nil {
			assert.Equal(t, test.expErr, err)
			continue
		}

		if !test.isoGroup.UsePodAntiAffinity {
			assert.Nil(t, antiaffinity)
			continue
		}

		terms := antiaffinity.RequiredDuringSchedulingIgnoredDuringExecution

		expTerms := []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      labels.Component,
							Operator: "In",
							Values:   []string{labels.ComponentM3DBNode},
						},
					},
				},
				TopologyKey: test.expTerm,
			},
		}

		assert.Equal(t, expTerms, terms)
	}
}
