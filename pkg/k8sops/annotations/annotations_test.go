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

package annotations

import (
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestGenerateBaseAnnotations(t *testing.T) {
	cluster := &myspec.M3DBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-foo",
		},
	}

	annotations := BaseAnnotations(cluster)
	expAnnotations := map[string]string{
		"operator.m3db.io/app":     "m3db",
		"operator.m3db.io/cluster": "cluster-foo",
	}

	assert.Equal(t, expAnnotations, annotations)

	cluster.Spec.Annotations = map[string]string{"foo": "bar"}
	annotations = BaseAnnotations(cluster)
	expAnnotations["foo"] = "bar"

	assert.Equal(t, expAnnotations, annotations)
}

func TestGeneratePodAnnotations(t *testing.T) {
	cluster := &myspec.M3DBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-foo",
		},
		Spec: myspec.ClusterSpec{
			PodMetadata: metav1.ObjectMeta{
				Annotations: map[string]string{
					"pod-annotation": "some-annotation",
				},
			},
		},
	}

	annotations := PodAnnotations(cluster)
	expAnnotations := map[string]string{
		"operator.m3db.io/app":     "m3db",
		"operator.m3db.io/cluster": "cluster-foo",
		"pod-annotation":           "some-annotation",
	}

	assert.Equal(t, expAnnotations, annotations)

	cluster.Spec.Annotations = map[string]string{"foo": "bar"}
	annotations = PodAnnotations(cluster)
	expAnnotations["foo"] = "bar"
	expAnnotations["pod-annotation"] = "some-annotation"

	assert.Equal(t, expAnnotations, annotations)
}
