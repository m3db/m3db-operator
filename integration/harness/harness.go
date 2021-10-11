// +build integration

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

package harness

import (
	"context"
	"fmt"

	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"go.uber.org/zap"
)

const (
	defaultNamespace              = "m3db-e2e-test-1"
	defaultClusterRoleName        = "operator-e2e-test"
	defaultClusterRoleBindingName = defaultClusterRoleName
	proxyBaseURLFmt               = "http://localhost:8001/api/v1/namespaces/%s/services/%s:%s/proxy"
)

// Global is the global read-only harness used by all tests.
var Global *Harness

// Harness encapsulates all dependencies to run a test.
type Harness struct {
	Logger     *zap.Logger
	KubeClient kubernetes.Interface
	CRDClient  clientset.Interface
	ExtClient  extclient.Interface
	Namespace  string
	Flags      Flags
}

type suiteOpts struct {
	flags  Flags
	logger *zap.Logger
}

func buildSuite(opts *suiteOpts) (*Harness, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", opts.flags.KubeConfig)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	crdClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	extClient, err := extclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	h := &Harness{
		Logger:     opts.logger,
		KubeClient: kubeClient,
		CRDClient:  crdClient,
		ExtClient:  extClient,
		Namespace:  defaultNamespace,
		Flags:      opts.flags,
	}

	return h, nil
}

func (h *Harness) ensureNamespace(ctx context.Context) error {
	ns, err := h.KubeClient.CoreV1().Namespaces().Get(ctx, h.Namespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return h.createNamespace(ctx, defaultNamespace)
		}
		return err
	}

	if ns.Status.Phase != corev1.NamespaceActive {
		return fmt.Errorf("namespace %s is in phase != Active (%s)", h.Namespace, ns.Status.Phase)
	}

	h.Logger.Info("found namespace", zap.String("namespace", ns.Name))
	return nil
}

func (h *Harness) setupSuite(ctx context.Context) error {
	if err := h.ensureNamespace(ctx); err != nil {
		return err
	}

	if err := h.installOperator(ctx); err != nil {
		return err
	}

	if err := h.createEtcdCluster(ctx); err != nil {
		return err
	}

	return nil
}

func (h *Harness) createNamespace(ctx context.Context, ns string) error {
	if h.Flags.SkipCreateNamespace {
		return nil
	}

	h.Logger.Info("creating namespace", zap.String("namespace", ns))
	_, err := h.KubeClient.CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		},
		metav1.CreateOptions{},
	)
	return err
}

func (h *Harness) teardownSuite(ctx context.Context) error {
	if !h.Flags.SkipDeleteRBAC {
		h.Logger.Info("deleting clusterrole", zap.String("clusterrole", defaultClusterRoleName))
		err := h.KubeClient.RbacV1().ClusterRoles().Delete(ctx, defaultClusterRoleName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		h.Logger.Info("deleting clusterrolebinding", zap.String("clusterrolebinding", defaultClusterRoleBindingName))
		err = h.KubeClient.RbacV1().ClusterRoleBindings().
			Delete(ctx, defaultClusterRoleBindingName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	if h.Flags.SkipDeleteNamespace {
		return nil
	}

	h.Logger.Info("deleting namespace")
	err := h.KubeClient.CoreV1().Namespaces().Delete(ctx, h.Namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	h.Logger.Info("namespace deleted")
	return nil
}
