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

	"github.com/stretchr/testify/require"
)

func TestStatefulSet(t *testing.T) {
	fixture := getFixture("testM3DBCluster.yaml", t)
	k, err := newFakeK8sops()
	require.Nil(t, err)

	ssName := StatefulSetName(fixture.GetName(), 0)
	require.Equal(t, "m3db-cluster-rep0", ssName)

	getSS, err := k.GetStatefulSet(fixture, ssName)
	require.Nil(t, err)
	require.NotNil(t, getSS)

	_, err = k.GetStatefulSets(fixture, k.LabelSelector("fake", "fake"))
	require.NotNil(t, err)
	err = k.DeleteStatefulSets(fixture, k.LabelSelector("fake", "fake"))
	require.NotNil(t, err)
}
