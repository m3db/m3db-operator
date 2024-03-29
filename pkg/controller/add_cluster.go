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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/util/eventer"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"go.uber.org/zap"
)

var (
	errEmptyConfigMap = errors.New("ConfigMapName cannot be empty if non-nil")
)

func (c *M3DBController) ensureServices(ctx context.Context, cluster *myspec.M3DBCluster) error {
	coordSvc, err := m3db.GenerateCoordinatorService(cluster)
	if err != nil {
		return err
	}

	m3dbSvc, err := m3db.GenerateM3DBService(cluster)
	if err != nil {
		return err
	}

	services := []*corev1.Service{
		coordSvc,
		m3dbSvc,
	}

	for _, svc := range services {
		err = c.k8sclient.EnsureService(ctx, cluster, svc)
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
func (c *M3DBController) ensureConfigMap(ctx context.Context, cluster *myspec.M3DBCluster) error {
	if cluster.Spec.ConfigMapName != nil {
		if *cluster.Spec.ConfigMapName == "" {
			return errEmptyConfigMap
		}
		// Nothing to do if user specified config map.
		return nil
	}

	wantCM, err := m3db.GenerateDefaultConfigMap(cluster)
	if err != nil {
		return err
	}

	cmClient := c.kubeClient.CoreV1().ConfigMaps(cluster.Namespace)

	// Check if there is a configmap that exists. If so, overwrite it with the
	// current templated out config. Otherwise, create one.
	cm, err := cmClient.Get(ctx, wantCM.Name, metav1.GetOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}

		// If the config doesn't exist, create it.
		_, err := cmClient.Create(ctx, wantCM, metav1.CreateOptions{})
		return err
	}

	// Make a copy of the configmap to not corrupt cache.
	cm = cm.DeepCopy()

	// Found an existing config map, check if we need to update its contents.
	if reflect.DeepEqual(cm.Data, wantCM.Data) {
		c.logger.Debug("config maps equal, nothing to do",
			zap.String("namespace", cluster.Namespace),
			zap.String("cluster", cluster.Name),
		)
		return nil
	}

	cmBytes, err := json.Marshal(cm)
	if err != nil {
		return err
	}

	cm.Data = wantCM.Data

	cmModifiedBytes, err := json.Marshal(cm)
	if err != nil {
		return err
	}

	patchBytes, err := jsonpatch.CreateMergePatch(cmBytes, cmModifiedBytes)
	if err != nil {
		return err
	}

	_, err = cmClient.Patch(ctx, cm.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		c.logger.Error("error updating configmap", zap.Error(err))
	} else {
		c.logger.Info("updated configmap data",
			zap.String("namespace", cluster.Namespace),
			zap.String("cluster", cluster.Name),
			zap.String("configmap", cm.Name),
		)
	}

	return err
}
