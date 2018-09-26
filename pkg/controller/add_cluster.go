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
	"fmt"

	plc "github.com/m3db/m3/src/query/api/v1/handler/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3x/ident"

	"go.uber.org/zap"
)

const (
	// defaults for placement init request
	_defaultM3DBPort = 9000

	_failedToInitM3DB = "m3 db failed to initialize"
	_initialzingM3DB  = "m3 db is initializing"
	_upAndRunningM3DB = "m3 db is up and running"

	_M3DBSvcName          = "m3dbnode"
	_M3CoordinatorSvcName = "m3coordinator"
)

func (c *Controller) addM3DBCluster(cluster *myspec.M3DBCluster) error {

	redStatus := myspec.M3DBStatus{
		State:   myspec.RedState,
		Message: _failedToInitM3DB,
	}

	// Refresh cluster map with latest state
	if err := c.refreshClusters(); err != nil {
		c.logger.Error("error refreshing cluster", zap.Error(err))
		return err
	}
	// Ensure local copies of the cluster object from the internal map
	cls := c.clusters[cluster.GetName()]
	status := myspec.M3DBStatus{
		State:   myspec.YellowState,
		Message: _initialzingM3DB,
	}
	if err := c.updateM3DBStatus(cls.M3DBCluster, status); err != nil {
		return err
	}
	// Create M3DB node service defined in service configuration
	svcCfg, found := c.getServiceConfig(_M3DBSvcName, cls.M3DBCluster.Spec.ServiceConfigurations)
	if !found {
		return fmt.Errorf("%s service was not found", _M3DBSvcName)
	}
	if err := c.k8sclient.EnsureService(cls.M3DBCluster, svcCfg); err != nil {
		redStatus.Message = fmt.Sprintf("%s: %v", redStatus.Message, err)
		if err := c.updateM3DBStatus(cls.M3DBCluster, redStatus); err != nil {
			return err
		}
		return err
	}
	// Create M3Coodinator service defined in service configuration
	svcCfg, found = c.getServiceConfig(_M3CoordinatorSvcName, cls.M3DBCluster.Spec.ServiceConfigurations)
	if !found {
		return fmt.Errorf("%s service was not found", _M3DBSvcName)
	}
	if err := c.k8sclient.EnsureService(cls.M3DBCluster, svcCfg); err != nil {
		redStatus.Message = fmt.Sprintf("%s: %v", redStatus.Message, err)
		if err := c.updateM3DBStatus(cls.M3DBCluster, redStatus); err != nil {
			return err
		}
		return err
	}
	svcCfg, found = c.getServiceConfig(_M3DBSvcName, cls.M3DBCluster.Spec.ServiceConfigurations)
	if !found {
		return fmt.Errorf("%s service was not found", _M3DBSvcName)
	}
	// TODO(PS) replace statefulsets with pods instead
	if err := c.k8sclient.EnsureStatefulSets(cls.M3DBCluster, svcCfg); err != nil {
		redStatus.Message = fmt.Sprintf("%s: %v", redStatus.Message, err)
		if err := c.updateM3DBStatus(cls.M3DBCluster, redStatus); err != nil {
			return err
		}
		return err

	}
	if err := c.EnsureNamespace(cls.M3DBCluster); err != nil {
		redStatus.Message = fmt.Sprintf("%s: %v", redStatus.Message, err)
		if err := c.updateM3DBStatus(cls.M3DBCluster, redStatus); err != nil {
			return err
		}
		return err
	}
	if err := c.EnsurePlacement(cls.M3DBCluster); err != nil {
		redStatus.Message = fmt.Sprintf("%s: %v", redStatus.Message, err)
		if err := c.updateM3DBStatus(cls.M3DBCluster, redStatus); err != nil {
			return err
		}
		return err
	}
	status = myspec.M3DBStatus{
		State:   myspec.GreenState,
		Message: _upAndRunningM3DB,
	}
	return c.updateM3DBStatus(cls.M3DBCluster, status)
}

func (c *Controller) getServiceConfig(serviceName string, svcCfgs []myspec.ServiceConfiguration) (myspec.ServiceConfiguration, bool) {
	for _, svcCfg := range svcCfgs {
		if svcCfg.Name == serviceName {
			return svcCfg, true
		}
	}
	return myspec.ServiceConfiguration{}, false
}

// EnsurePlacement ensures that a placement exists otherwise create one
func (c *Controller) EnsurePlacement(cluster *myspec.M3DBCluster) error {
	// Get placement
	_, err := c.placementClient.Get()
	if err == m3admin.ErrNotFound {
		placementInitRequest := &admin.PlacementInitRequest{
			NumShards:         cluster.Spec.NumberOfShards,
			ReplicationFactor: cluster.Spec.ReplicationFactor,
		}
		placementDetails, err := c.k8sclient.GetPlacementDetails(cluster)
		if err != nil {
			return err
		}
		for hostname, zone := range placementDetails {
			fqdnHostname := fmt.Sprintf("%s.%s", hostname, plc.DefaultServiceName)
			instance := &placementpb.Instance{
				Id:             hostname,
				IsolationGroup: zone,
				Zone:           plc.DefaultServiceZone,
				Weight:         100, // TODO(PS) Remove once [PR](https://github.com/m3db/m3/pull/901) is merged
				Hostname:       fqdnHostname,
				Endpoint:       fmt.Sprintf("%s:%d", fqdnHostname, _defaultM3DBPort),
				Port:           _defaultM3DBPort,
			}
			placementInitRequest.Instances = append(placementInitRequest.Instances, instance)
		}
		if err := c.placementClient.Init(placementInitRequest); err != nil {
			c.logger.Error("failed to apply placement", zap.Error(err))
			return err
		}
	} else if err != nil {
		c.logger.Error("failed to apply placement", zap.Error(err))
		return err
	}
	return nil
}

// EnsureNamespace will retrieve current namespaces to ensure one matches
// the cluster name or create a new namespace to match the cluster name
func (c *Controller) EnsureNamespace(cluster *myspec.M3DBCluster) error {
	// Get namespace
	namespaces, err := c.namespaceClient.List()
	if err != nil {
		c.logger.Error("failed to get namespace ", zap.Error(err))
		return err
	}
	for _, md := range namespaces {
		if md.ID().Equal(ident.StringID(cluster.GetName())) {
			c.logger.Info("namespace found", zap.String("ns", md.ID().String()))
			return nil
		}
	}
	if err = c.namespaceClient.Create(cluster.GetObjectMeta().GetName()); err != nil {
		c.logger.Error("failed to create namespace", zap.Error(err))
		return err
	}
	return nil
}
