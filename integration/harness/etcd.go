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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"

	"go.uber.org/zap"
)

const (
	etcdHealthInterval = 10 * time.Second
	etcdHealthTimeout  = 5 * time.Minute
)

func (h *Harness) createEtcdCluster(ctx context.Context) error {
	f, err := os.Open("../manifests/etcd.yaml")
	if err != nil {
		return err
	}

	reader := yaml.NewYAMLOrJSONDecoder(f, 2048)

	headlessSvc := &corev1.Service{}
	if err := reader.Decode(headlessSvc); err != nil {
		return err
	}

	clusterSvc := &corev1.Service{}
	if err := reader.Decode(clusterSvc); err != nil {
		return err
	}

	statefulSet := &appsv1.StatefulSet{}
	if err := reader.Decode(statefulSet); err != nil {
		return err
	}

	for _, svc := range []*corev1.Service{
		headlessSvc,
		clusterSvc,
	} {
		h.Logger.Info("creating etcd service", zap.String("service", svc.Name))
		_, err := h.KubeClient.CoreV1().Services(h.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	h.Logger.Info("creating etcd statefulset", zap.String("statefulset", statefulSet.Name))
	_, err = h.KubeClient.AppsV1().StatefulSets(h.Namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if err := h.waitEtcdHealth(etcdHealthInterval, etcdHealthTimeout); err != nil {
		return err
	}

	return nil
}

func (h *Harness) waitEtcdHealth(interval, timeout time.Duration) error {
	return wait.Poll(interval, timeout, wait.ConditionFunc(func() (bool, error) {
		err := h.checkEtcdHealth()
		if err != nil {
			h.Logger.Error("error checking etcd health", zap.Error(err))
			return false, nil
		}
		return true, nil
	}))
}

func (h *Harness) checkEtcdHealth() error {
	url := fmt.Sprintf(proxyBaseURLFmt, h.Namespace, "etcd-cluster", "2379") + "/health"

	h.Logger.Info("checking etcd health", zap.String("url", url))

	health := struct {
		Health string `json:"health"`
	}{}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return err
	}

	if health.Health != "true" {
		return errors.New("unhealthy response from cluster")
	}

	return nil
}
