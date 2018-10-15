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
	genclient "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"

	"go.uber.org/zap"
)

// Option provides an interface that can be used for setter options with the
// constructor
type Option interface {
	execute(*k8sops) error
}

type optionFn func(k *k8sops) error

func (fn optionFn) execute(k *k8sops) error {
	return fn(k)
}

// WithLogger is a setter to override the default logger
func WithLogger(logger *zap.Logger) Option {
	return optionFn(func(k *k8sops) error {
		k.logger = logger
		return nil
	})
}

// WithCRDClient is a setter to override the default CRD client
func WithCRDClient(crdClient genclient.Interface) Option {
	return optionFn(func(k *k8sops) error {
		k.crdClient = crdClient
		return nil
	})
}

// WithKClient is a setter to override the default Kubernetes client
func WithKClient(kclient kubernetes.Interface) Option {
	return optionFn(func(k *k8sops) error {
		k.kclient = kclient
		return nil
	})
}

// WithExtClient is a setter to override the default Extensions Client
func WithExtClient(extClient apiextensionsclient.Interface) Option {
	return optionFn(func(k *k8sops) error {
		k.kubeExt = extClient
		return nil
	})
}

// k8sops defines the kube object
type k8sops struct {
	crdClient genclient.Interface
	kclient   kubernetes.Interface
	kubeExt   apiextensionsclient.Interface
	logger    *zap.Logger
}

// New creates a new instance of k8sops
func New(opts ...Option) (K8sops, error) {
	k8 := &k8sops{}
	for _, o := range opts {
		if err := o.execute(k8); err != nil {
			return nil, err
		}
	}

	return k8, nil
}
