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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureService_Base(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	k8sops, err := newFakeK8sops()
	require.NoError(t, err)

	c := &Controller{
		k8sclient: k8sops,
	}

	err = c.ensureServices(nil)
	assert.Error(t, err)

	err = c.ensureServices(cluster)
	assert.NoError(t, err)

	for _, svcName := range []string{"m3dbnode-cluster-simple", "m3coordinator-cluster-simple"} {
		svc, err := k8sops.GetService(cluster, svcName)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	}
}

func TestEnsureService_Custom(t *testing.T) {
	cluster := getFixture("cluster-services.yaml", t)
	k8sops, err := newFakeK8sops()
	require.NoError(t, err)

	c := &Controller{
		k8sclient: k8sops,
	}

	err = c.ensureServices(cluster)
	assert.NoError(t, err)

	for _, svcName := range []string{"m3dbnode-cluster-services", "custom-svc"} {
		svc, err := k8sops.GetService(cluster, svcName)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	}

	_, err = k8sops.GetService(cluster, "m3coordinator-cluster-services")
	assert.Error(t, err)
}
