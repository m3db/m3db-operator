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
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"

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
	kclient                    k8sops.K8sops
	crdClient                  clientset.Interface
	kubeClient                 kubernetes.Interface
	kubeInformerFactory        kubeinformers.SharedInformerFactory
	m3dbClusterInformerFactory informers.SharedInformerFactory
	namespaceClient            namespace.Client
	placementClient            placement.Client
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
func WithKClient(kc k8sops.K8sops) Option {
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

// WithM3DBClusterInformerFactory sets the factory for our custom type.
func WithM3DBClusterInformerFactory(f informers.SharedInformerFactory) Option {
	return optionFn(func(o *options) {
		o.m3dbClusterInformerFactory = f
	})
}

// WithNamespaceClient sets a custom m3db namespace client. If unset a default
// in-cluster client will be created.
func WithNamespaceClient(cl namespace.Client) Option {
	return optionFn(func(o *options) {
		o.namespaceClient = cl
	})
}

// WithPlacementClient sets the m3db placement client. If unset a default
// in-cluster client will be created.
func WithPlacementClient(cl placement.Client) Option {
	return optionFn(func(o *options) {
		o.placementClient = cl
	})
}

// Validate ensures the configured options are valid. Specifically, if any
// fields except the Logger, PlacementClient, and NamespaceClient are nil the
// options will be rejected.
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
	}

	return nil
}
