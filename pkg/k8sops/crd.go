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
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspecv1 "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	"go.uber.org/zap"
)

// ListM3DBCluster  will list all the CRDS for M3DBClusters in all namespaces
func (k *k8sops) ListM3DBCluster() (*myspecv1.M3DBClusterList, error) {
	// Get existing clusters
	currentClusters, err := k.crdClient.OperatorV1().M3DBClusters(v1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		k.logger.Error("could not get list of clusters", zap.Error(err))
		return nil, err
	}
	return currentClusters, nil
}

// GetM3DBCluster will get the M3DBCluster CRD
func (k *k8sops) GetM3DBCluster(namespace, name string) (*myspecv1.M3DBCluster, error) {
	crd, err := k.crdClient.OperatorV1().M3DBClusters(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		k.logger.Error("could not get cluster", zap.Error(err))
		return nil, err
	}
	return crd, nil
}

// GetCRD will get a CRD
func (k *k8sops) GetCRD(name string) (*apiextensionsv1beta1.CustomResourceDefinition, error) {
	crd, err := k.kubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, metav1.GetOptions{})
	if err != nil {
		k.logger.Error("could not get CRD", zap.Error(err))
		return nil, err
	}
	return crd, nil
}

// CreateCRD checks if M3DB CRD exists. If not, create
func (k *k8sops) CreateCRD(name string) error {
	crd, err := k.GetCRD(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			k.logger.Debug("crd is missing, creating it")
			crdObject := k.GenerateCRD()
			crd, err := k.kubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crdObject)
			if err != nil {
				panic(err)
			}
			// wait for CRD being established
			err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
				createdCRD, err := k.kubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				for _, cond := range createdCRD.Status.Conditions {
					switch cond.Type {
					case apiextensionsv1beta1.Established:
						if cond.Status == apiextensionsv1beta1.ConditionTrue {
							return true, nil
						}
					case apiextensionsv1beta1.NamesAccepted:
						if cond.Status == apiextensionsv1beta1.ConditionFalse {
							return false, fmt.Errorf("name conflict: %v", cond.Reason)
						}
					}
				}
				return false, nil
			})

			if err != nil {
				deleteErr := k.kubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(myspec.Name, nil)
				if deleteErr != nil {
					return errors.NewAggregate([]error{err, deleteErr})
				}
				return err
			}

			k.logger.Info("CRD created")
		} else {
			panic(err)
		}
	} else {
		k.logger.Info("CRD already exists", zap.String("name", crd.ObjectMeta.Name))
	}
	return nil
}

// UpdateCRD will update a CRD
func (k *k8sops) UpdateCRD(cluster *myspecv1.M3DBCluster) (*myspecv1.M3DBCluster, error) {
	updated, err := k.crdClient.OperatorV1().M3DBClusters(cluster.GetNamespace()).Update(cluster)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// NewListWatcher will provide a list watcher
func (k *k8sops) NewListWatcher() *cache.ListWatch {
	return cache.NewListWatchFromClient(k.crdClient.OperatorV1().RESTClient(), myspec.ResourcePlural, v1.NamespaceAll, fields.Everything())
}
