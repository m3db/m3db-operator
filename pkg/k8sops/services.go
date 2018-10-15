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
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.uber.org/zap"
)

// GetService simply gets a service by name
func (k *k8sops) GetService(cluster *myspec.M3DBCluster, name string) (*v1.Service, error) {
	service, err := k.kclient.CoreV1().Services(cluster.GetNamespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return service, nil
}

// DeleteService simply deletes a service by name
func (k *k8sops) DeleteService(cluster *myspec.M3DBCluster, name string) error {
	k.logger.Info("deleting service", zap.String("service", name))
	return k.kclient.CoreV1().Services(cluster.GetNamespace()).Delete(name, &metav1.DeleteOptions{})
}

// EnsureService will create a service by name if it doesn't exist
func (k *k8sops) EnsureService(cluster *myspec.M3DBCluster, svc *v1.Service) error {
	_, err := k.GetService(cluster, svc.Name)
	if errors.IsNotFound(err) {
		k.logger.Info("service doesn't exist, creating it", zap.String("service", svc.Name))
		selfRef := metav1.NewControllerRef(cluster, schema.GroupVersionKind{
			Group:   myspec.SchemeGroupVersion.Group,
			Version: myspec.SchemeGroupVersion.Version,
			Kind:    "m3dbcluster",
		})
		svc.SetOwnerReferences([]metav1.OwnerReference{*selfRef})
		if _, err := k.kclient.CoreV1().Services(cluster.GetNamespace()).Create(svc); err != nil {
			return err
		}
		k.logger.Info("ensured service is created", zap.String("service", svc.GetName()))
	} else if errors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}
