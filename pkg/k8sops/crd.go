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

package k8sops

import (
	"fmt"
	"net/http"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	pkgerrors "github.com/pkg/errors"
	"go.uber.org/zap"
)

func (k *k8sops) CreateOrUpdateCRD(name string, enableValidation bool) error {
	if name != myspec.Name {
		return fmt.Errorf("unrecognized CRD name '%s'", name)
	}

	crdClient := k.kubeExt.ApiextensionsV1beta1().CustomResourceDefinitions()
	curCRD, err := crdClient.Get(name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return pkgerrors.WithMessagef(err, "could not fetch CRD '%s'", name)
	}

	newCRD := GenerateCRD(enableValidation)
	if apierrors.IsNotFound(err) {
		_, err := crdClient.Create(newCRD)
		if err != nil {
			return pkgerrors.WithMessagef(err, "error creating CRD '%s'", name)
		}
		k.logger.Info("created CRD", zap.String("name", name))
		return nil
	}

	// CRD exists, update it
	curCRD.Spec = newCRD.Spec
	newCRD, err = crdClient.Update(curCRD)
	if err != nil {
		return pkgerrors.WithMessagef(err, "error updating CRD '%s'", name)
	}

	k.logger.Info("updated CRD",
		zap.String("name", name),
		zap.String("oldRV", curCRD.ResourceVersion),
		zap.String("newRV", newCRD.ResourceVersion),
	)

	return k.waitForCRDReady(name)
}

// waitForCRDReady waits until we can list resources of the given type,
// indicating that the resource is ready.
func (k *k8sops) waitForCRDReady(name string) error {
	if name != myspec.Name {
		return fmt.Errorf("unrecognized CRD name '%s'", name)
	}

	// wait until we can list resources of our type without getting resource not
	// found errors.
	err := wait.Poll(2*time.Second, 5*time.Minute, func() (bool, error) {
		_, err := k.crdClient.OperatorV1alpha1().M3DBClusters(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err == nil {
			return true, nil
		}

		if se, ok := err.(*apierrors.StatusError); ok {
			if se.Status().Code == http.StatusNotFound {
				return false, nil
			}
		}

		return false, pkgerrors.WithMessagef(err, "failed to list crd '%s'", name)
	})

	return pkgerrors.WithMessagef(err, "timed out waiting for CRD '%s' to be registered'", name)
}
