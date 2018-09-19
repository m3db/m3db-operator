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

func (c *Controller) updateM3DBCluster(cluster *myspec.M3DBCluster) error {

	oldClusters := make(map[string]Cluster)
	for clusterName, cluster := range c.clusters {
		oldClusters[clusterName] = Cluster{M3DBCluster: cluster.M3DBCluster.DeepCopy()}
	}

	if err := c.refreshClusters(); err != nil {
		c.logger.Error("failed to refresh cluster, can not compare modified state", zap.Error(err))
		return err
	}

	oldCluster := oldClusters[cluster.GetName()]
	currentCluster := c.clusters[cluster.GetName()]

	for i, ig := range currentCluster.M3DBCluster.Spec.IsolationGroups {
		oldIG := oldCluster.M3DBCluster.Spec.IsolationGroups[i]
		if oldIG.Name == ig.Name {
			if oldIG.NumInstances != ig.NumInstances {
				ssName := c.k8sclient.StatefulSetName(cluster.GetName(), ig.Name)
				ss, err := c.k8sclient.GetStatefulSet(cluster, ssName)
				if err != nil {
					return err
				}
				ssCopy := ss.DeepCopy()
				incrementAmount := oldIG.NumInstances - ig.NumInstances
				*ssCopy.Spec.Replicas += incrementAmount
				ss, err = c.k8sclient.UpdateStatefulSet(cluster, ssCopy)
				if err != nil {
					return err
				}
				c.logger.Info("instance amount changed", zap.Any("ss", ss))
			}
		}
	}
	return nil
}
