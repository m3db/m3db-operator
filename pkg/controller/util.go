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
