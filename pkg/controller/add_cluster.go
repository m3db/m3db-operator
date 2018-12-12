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

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"go.uber.org/zap"
)

const (
	// defaults for placement init request
	_defaultM3DBPort = 9000
	_defaultZone     = "embedded"
)

var (
	errEmptyConfigMap = errors.New("ConfigMapName cannot be empty if non-nil")
)

// EnsurePlacement ensures that a placement exists otherwise create one
func (c *Controller) EnsurePlacement(cluster *myspec.M3DBCluster) error {
	// Get placement
	plClient := c.adminClient.placementClientForCluster(cluster)
	_, err := plClient.Get()
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
			fqdnHostname := fmt.Sprintf("%s.%s", hostname, cluster.Namespace)
			instance := &placementpb.Instance{
				Id:             hostname,
				IsolationGroup: zone,
				Zone:           _defaultZone,
				Weight:         100, // TODO(PS) Remove once [PR](https://github.com/m3db/m3/pull/901) is merged
				Hostname:       fqdnHostname,
				Endpoint:       fmt.Sprintf("%s:%d", fqdnHostname, _defaultM3DBPort),
				Port:           _defaultM3DBPort,
			}
			placementInitRequest.Instances = append(placementInitRequest.Instances, instance)
		}
		if err := plClient.Init(placementInitRequest); err != nil {
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
	coordSvc, err := k8sops.GenerateCoordinatorService(cluster)
	if err != nil {
		return err
	}

	m3dbSvc, err := k8sops.GenerateM3DBService(cluster)
	if err != nil {
		return err
	}

	services := []*corev1.Service{
		coordSvc,
		m3dbSvc,
	}

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

// ensureConfigMap creates the default configmap for the cluster if none is
// specified in the cluster spec.
func (c *Controller) ensureConfigMap(cluster *myspec.M3DBCluster) error {
	if cluster.Spec.ConfigMapName != nil {
		if *cluster.Spec.ConfigMapName == "" {
			return errEmptyConfigMap
		}
		// Nothing to do if user specified config map.
		return nil
	}

	cm, err := k8sops.GenerateDefaultConfigMap(cluster)
	if err != nil {
		return err
	}

	_, err = c.kubeClient.CoreV1().ConfigMaps(cluster.Namespace).Create(cm)
	if kerrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
