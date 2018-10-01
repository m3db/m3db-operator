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
	"errors"
	"fmt"

	plc "github.com/m3db/m3/src/query/api/v1/handler/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3x/ident"
	corev1 "k8s.io/api/core/v1"

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

// EnsureNamespace will retrieve current namespaces to ensure one matches the
// cluster name or create a new namespace to match the cluster name. Returns a
// bool indicating whether a namespace was created and any errors encountered.
func (c *Controller) EnsureNamespace(cluster *myspec.M3DBCluster) (bool, error) {
	// Get namespace
	namespaces, err := c.namespaceClient.List()
	if err != nil {
		c.logger.Error("failed to get namespace ", zap.Error(err))
		return false, err
	}
	for _, md := range namespaces {
		if md.ID().Equal(ident.StringID(cluster.GetName())) {
			c.logger.Info("namespace found", zap.String("ns", md.ID().String()))
			return false, nil
		}
	}
	if err = c.namespaceClient.Create(cluster.GetObjectMeta().GetName()); err != nil {
		c.logger.Error("failed to create namespace", zap.Error(err))
		return false, err
	}
	return true, nil
}

func (c *Controller) ensureServices(cluster *myspec.M3DBCluster) error {
	// TODO(schallert): support updating service spec, not sure if this only
	// handles creation.
	m3dbSvc, found := c.getServiceConfig(_M3DBSvcName, cluster.Spec.ServiceConfigurations)
	if !found {
		err := errors.New("m3db service not found in spec")
		c.logger.Error(err.Error())
		c.recorder.Event(cluster, corev1.EventTypeWarning, "NoServiceConfig", "could not create m3db service, missing service config in spec")
		return err
	}

	if err := c.k8sclient.EnsureService(cluster, m3dbSvc); err != nil {
		err = fmt.Errorf("error creating m3db service: %v", err)
		c.logger.Error(err.Error())
		c.recorder.Event(cluster, corev1.EventTypeWarning, "ServiceCreateError", err.Error())
		return err
	}

	coordSvc, found := c.getServiceConfig(_M3CoordinatorSvcName, cluster.Spec.ServiceConfigurations)
	if !found {
		err := errors.New("coordinator service not found in spec")
		c.logger.Error(err.Error())
		c.recorder.Event(cluster, corev1.EventTypeWarning, "NoServiceConfig", "could not create m3coordinator service, missing service config in spec")
		return err
	}

	if err := c.k8sclient.EnsureService(cluster, coordSvc); err != nil {
		err = fmt.Errorf("error creating m3coordinator service: %v", err)
		c.logger.Error(err.Error())
		c.recorder.Event(cluster, corev1.EventTypeWarning, "ServiceCreateError", err.Error())
		return err
	}

	return nil
}
