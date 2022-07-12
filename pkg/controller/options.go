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

	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	informers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

// Option provides configuration of a controller.
type Option interface {
	execute(*options)
}

type options struct {
	logger                     *zap.Logger
	scope                      tally.Scope
	kclient                    m3db.K8sops
	podIDProvider              podidentity.Provider
	crdClient                  clientset.Interface
	kubeClient                 kubernetes.Interface
	kubeInformerFactory        kubeinformers.SharedInformerFactory
	filteredInformerFactory    kubeinformers.SharedInformerFactory
	m3dbClusterInformerFactory informers.SharedInformerFactory
	kubectlProxy               bool
}

type optionFn func(o *options)

func (fn optionFn) execute(o *options) {
	fn(o)
}

// WithScope sets the telemetry scope
func WithScope(s tally.Scope) Option {
	return optionFn(func(o *options) {
		o.scope = s
	})
}

// WithLogger sets a logger.
func WithLogger(l *zap.Logger) Option {
	return optionFn(func(o *options) {
		o.logger = l
	})
}

// WithKClient sets the k8sops client.
func WithKClient(kc m3db.K8sops) Option {
	return optionFn(func(o *options) {
		o.kclient = kc
	})
}

// WithCRDClient sets the client of our custom type.
func WithCRDClient(cl clientset.Interface) Option {
	return optionFn(func(o *options) {
		o.crdClient = cl
	})
}

// WithKubeClient sets the base Kubernetes client.
func WithKubeClient(cl kubernetes.Interface) Option {
	return optionFn(func(o *options) {
		o.kubeClient = cl
	})
}

// WithKubeInformerFactory sets the Kubernetes base type informer factory.
func WithKubeInformerFactory(f kubeinformers.SharedInformerFactory) Option {
	return optionFn(func(o *options) {
		o.kubeInformerFactory = f
	})
}

// WithFilteredInformerFactory sets the Filteredrnetes base type informer factory.
func WithFilteredInformerFactory(f kubeinformers.SharedInformerFactory) Option {
	return optionFn(func(o *options) {
		o.filteredInformerFactory = f
	})
}

// WithM3DBClusterInformerFactory sets the factory for our custom type.
func WithM3DBClusterInformerFactory(f informers.SharedInformerFactory) Option {
	return optionFn(func(o *options) {
		o.m3dbClusterInformerFactory = f
	})
}

// WithKubectlProxy sets whether to use a kubectl proxy for communicating with
// the cluster.
func WithKubectlProxy(use bool) Option {
	return optionFn(func(o *options) {
		o.kubectlProxy = use
	})
}

// WithPodIdentityProvider sets the pod identity provider.
func WithPodIdentityProvider(p podidentity.Provider) Option {
	return optionFn(func(o *options) {
		o.podIDProvider = p
	})
}

// Validate ensures the configured options are valid. Specifically, if any
// fields except the logger are nil the options will be rejected.
func (o *options) validate() error {
	switch {
	case o.kclient == nil:
		return errors.New("k8sops client cannot be nil")
	case o.crdClient == nil:
		return errors.New("CRD client cannot be nil")
	case o.kubeClient == nil:
		return errors.New("kubeclient cannot be nil")
	case o.kubeInformerFactory == nil:
		return errors.New("kubeInformerFactory cannot be nil")
	case o.m3dbClusterInformerFactory == nil:
		return errors.New("m3dbClusterInformerFactory cannot be nil")
	case o.podIDProvider == nil:
		return errors.New("pod ID provider cannot be nil")
	}

	return nil
}
