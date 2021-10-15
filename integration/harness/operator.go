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
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"go.uber.org/zap"
)

func (h *Harness) installOperator(ctx context.Context) error {
	f, err := os.Open("../manifests/operator.yaml")
	if err != nil {
		return err
	}

	reader := yaml.NewYAMLOrJSONDecoder(f, 2048)

	sa := &corev1.ServiceAccount{}
	role := &rbacv1.ClusterRole{}
	rb := &rbacv1.ClusterRoleBinding{}
	sts := &appsv1.StatefulSet{}

	for _, iface := range []interface{}{
		sa,
		role,
		rb,
		sts,
	} {
		if err := reader.Decode(iface); err != nil {
			return err
		}
	}

	h.Logger.Info("creating serviceaccount", zap.String("serviceaccount", sa.Name))
	if _, err := h.KubeClient.CoreV1().ServiceAccounts(h.Namespace).
		Create(ctx, sa, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		h.Logger.Info("found existing serviceaccount")
	}

	rc := h.KubeClient.RbacV1()

	if !h.Flags.SkipCreateRBAC {
		h.Logger.Info("creating clusterrole", zap.String("role", role.Name))
		if _, err := rc.ClusterRoles().Create(ctx, role, metav1.CreateOptions{}); err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			h.Logger.Info("found existing clusterrole")
		}

		h.Logger.Info("creating rolebinding", zap.String("rolebinding", rb.Name))
		if _, err := rc.ClusterRoleBindings().Create(ctx, rb, metav1.CreateOptions{}); err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			h.Logger.Info("found existing clusterrolebinding")
		}
	}

	h.Logger.Info("creating statefulset", zap.String("statefulset", sts.Name))
	_, err = h.KubeClient.AppsV1().StatefulSets(h.Namespace).Create(ctx, sts, metav1.CreateOptions{})
	return err
}
