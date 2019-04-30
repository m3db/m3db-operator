// +build integration

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

package e2e

import (
	"sort"
	"testing"
	"time"

	"github.com/m3db/m3db-operator/integration/harness"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	placementCheckInterval = 5 * time.Second
	placementCheckTimeout  = 5 * time.Minute
)

func TestCreateCluster_Regional(t *testing.T) {
	h := harness.Global

	cluster, err := h.CreateM3DBCluster("cluster-regional.yaml")
	require.NoError(t, err)

	cl, err := h.NewPlacementClient(cluster)
	require.NoError(t, err)

	err = wait.Poll(placementCheckInterval, placementCheckTimeout, func() (bool, error) {
		pl, err := cl.Get()
		if err != nil {
			h.Logger.Warn("error fetching placement", zap.Error(err))
			return false, nil
		}

		if pl.NumInstances() != 3 {
			h.Logger.Info("waiting for 3 instances to exist")
			return false, nil
		}

		isoGroups := []string{}
		for _, inst := range pl.Instances() {
			if !inst.IsAvailable() {
				h.Logger.Info("waiting for instance to be available", zap.String("instance", inst.ID()))
				return false, nil
			}
			isoGroups = append(isoGroups, inst.IsolationGroup())
		}

		sort.Strings(isoGroups)
		assert.Equal(t, []string{"group1", "group2", "group3"}, isoGroups)

		return true, nil
	})

	require.NoError(t, err)
}

func TestCreateCluster_Zonal(t *testing.T) {
	h := harness.Global

	cluster, err := h.CreateM3DBCluster("cluster-zonal.yaml")
	require.NoError(t, err)

	cl, err := h.NewPlacementClient(cluster)
	require.NoError(t, err)

	err = wait.Poll(placementCheckInterval, placementCheckTimeout, func() (bool, error) {
		pl, err := cl.Get()
		if err != nil {
			h.Logger.Warn("error fetching placement", zap.Error(err))
			return false, nil
		}

		if pl.NumInstances() != 3 {
			h.Logger.Info("waiting for 3 instances to exist")
			return false, nil
		}

		isoGroups := []string{}
		for _, inst := range pl.Instances() {
			if !inst.IsAvailable() {
				h.Logger.Info("waiting for instance to be available", zap.String("instance", inst.ID()))
				return false, nil
			}
			isoGroups = append(isoGroups, inst.IsolationGroup())
		}

		sort.Strings(isoGroups)
		assert.Equal(t, []string{"group1", "group2", "group3"}, isoGroups)

		return true, nil
	})

	require.NoError(t, err)
}
