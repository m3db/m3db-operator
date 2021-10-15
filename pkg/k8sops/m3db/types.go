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

package m3db

import (
	"context"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	v1 "k8s.io/api/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// K8sops provides an interface for various Kubernetes API calls
type K8sops interface {
	// CreateOrUpdateCRD creates the CRD if it does not exist, or updates it to
	// contain the latest spec if it does exist.
	CreateOrUpdateCRD(ctx context.Context, name string, enableValidation bool) error

	// GetService simply gets a service by name
	GetService(ctx context.Context, cluster *myspec.M3DBCluster, name string) (*v1.Service, error)

	// DeleteService simply deletes a service by name
	DeleteService(ctx context.Context, cluster *myspec.M3DBCluster, name string) error

	// EnsureService will create a service by name if it doesn't exist
	EnsureService(ctx context.Context, cluster *myspec.M3DBCluster, svc *v1.Service) error

	// Events returns an Event interface for a given namespace.
	Events(namespace string) typedcorev1.EventInterface
}
