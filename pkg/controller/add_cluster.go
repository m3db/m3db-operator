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

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	plc "github.com/m3db/m3/src/query/api/v1/handler/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"

	corev1 "k8s.io/api/core/v1"

	"go.uber.org/zap"
)

const (
	// defaults for placement init request
	_defaultM3DBPort = 9000
)

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
			fqdnHostname := fmt.Sprintf("%s.%s", hostname, plc.M3DBServiceName)
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

func (c *Controller) ensureServices(cluster *myspec.M3DBCluster) error {
	if cluster == nil {
		return errors.New("cluster cannot be nil")
	}

	// TODO(schallert): support updating service spec, not sure if this only
	// handles creation.

	services := []*corev1.Service{}
	if len(cluster.Spec.Services) != 0 {
		services = cluster.Spec.Services
	} else {
		coordSvc, err := k8sops.GenerateCoordinatorService(cluster)
		if err != nil {
			return err
		}
		services = append(services,
			coordSvc,
		)
	}

	m3dbSvc, err := k8sops.GenerateM3DBService(cluster)
	if err != nil {
		return err
	}

	services = append(services, m3dbSvc)

	for _, svc := range services {
		err = c.k8sclient.EnsureService(cluster, svc)
		if err != nil {
			err := fmt.Errorf("error creating service '%s': %v", svc.Name, err)
			c.recorder.WarningEvent(cluster, eventer.ReasonFailedCreate, err.Error())
			return err
		}
	}

	return nil
}
