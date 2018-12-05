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

package podidentity

import (
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider(t *testing.T, opts ...Option) Provider {
	p, err := NewProvider(opts...)
	require.NoError(t, err)
	return p
}

func TestNewProvider(t *testing.T) {
	p, err := NewProvider()
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestIdentity(t *testing.T) {
	tests := []struct {
		name      string
		pod       *corev1.Pod
		cluster   *myspec.M3DBCluster
		expID     *myspec.PodIdentity
		expErrAny bool
		expErr    error
	}{
		{
			name: "bad config unknown source",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-0",
					UID:  types.UID("foo"),
				},
			},
			cluster: &myspec.M3DBCluster{
				Spec: myspec.ClusterSpec{
					PodIdentityConfig: &myspec.PodIdentityConfig{
						Sources: []myspec.PodIdentitySource{
							myspec.PodIdentitySource("badsource"),
						},
					},
				},
			},
			expErrAny: true,
		},
		{
			name: "bad config empty name",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("foo"),
				},
			},
			cluster: &myspec.M3DBCluster{},
			expErr:  errEmptyPodSourceName,
		},
		{
			name: "bad config empty UID",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			cluster: &myspec.M3DBCluster{},
			expErr:  errEmptyPodSourceUID,
		},
		{
			name: "no config",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-0",
					UID:  types.UID("foo"),
				},
			},
			cluster: &myspec.M3DBCluster{},
			expID: &myspec.PodIdentity{
				Name: "pod-0",
				UID:  "foo",
			},
		},
		{
			name: "UID config",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("foo"),
				},
			},
			cluster: &myspec.M3DBCluster{
				Spec: myspec.ClusterSpec{
					PodIdentityConfig: &myspec.PodIdentityConfig{
						Sources: []myspec.PodIdentitySource{
							myspec.PodIdentitySourcePodUID,
						},
					},
				},
			},
			expID: &myspec.PodIdentity{
				UID: "foo",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := newTestProvider(t)
			id, err := p.Identity(test.pod, test.cluster)
			if test.expErrAny {
				assert.Error(t, err)
				return
			}
			if test.expErr != nil {
				assert.Equal(t, test.expErr, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expID, id)
		})
	}
}

func TestIdentityJSON(t *testing.T) {
	id := &myspec.PodIdentity{
		Name: "foo",
		UID:  "bar",
	}

	exp := `{"name":"foo","uid":"bar"}`
	res, err := IdentityJSON(id)
	assert.NoError(t, err)
	assert.Equal(t, exp, res)
}
