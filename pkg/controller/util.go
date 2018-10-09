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
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"go.uber.org/zap"
)

func (c *Controller) updateM3DBStatus(cluster *myspec.M3DBCluster, status myspec.M3DBStatus) error {
	// Get a fresh copy of the cluster to ensure it's not stale to write to.
	updatedCluster, err := c.k8sclient.GetM3DBCluster(cluster.GetNamespace(), cluster.GetName())
	if err != nil {
		c.logger.Error("failed to get latest M3DBCluster", zap.Error(err))
	}

	clusterCopy := updatedCluster.DeepCopy()
	clusterCopy.Status = status
	updated, err := c.k8sclient.UpdateCRD(clusterCopy)
	if err != nil {
		return err
	}
	c.logger.Info("updated M3DBCluster", zap.Any("status", updated.Status))
	return nil
}
