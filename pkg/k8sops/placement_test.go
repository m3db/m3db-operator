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

package k8sops

import (
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"

	"github.com/m3db/m3cluster/generated/proto/placementpb"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestPlacementInstanceFromPod(t *testing.T) {
	cluster := &myspec.M3DBCluster{}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}

	_, err := PlacementInstanceFromPod(cluster, pod)
	assert.Error(t, err)

	cluster.ObjectMeta.Name = "cluster-a"
	pod.ObjectMeta.Labels[labels.IsolationGroup] = "zone-a"
	pod.ObjectMeta.Name = "pod-a"

	expInst := &placementpb.Instance{
		Id:             "pod-a",
		IsolationGroup: "zone-a",
		Zone:           "embedded",
		Weight:         100,
		Hostname:       "pod-a.m3dbnode-cluster-a",
		Endpoint:       "pod-a.m3dbnode-cluster-a:9000",
		Port:           9000,
	}

	inst, err := PlacementInstanceFromPod(cluster, pod)
	assert.NoError(t, err)
	assert.Equal(t, expInst, inst)
}
