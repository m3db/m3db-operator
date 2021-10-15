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
	"path/filepath"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"go.uber.org/zap"
)

// CreateM3DBCluster creates an m3db cluster from a predefined manifest.
func (h *Harness) CreateM3DBCluster(ctx context.Context, filename string) (*myspec.M3DBCluster, error) {
	path := filepath.Join("../manifests", filename)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader := yaml.NewYAMLOrJSONDecoder(f, 2048)

	cluster := &myspec.M3DBCluster{}

	for _, iface := range []interface{}{
		cluster,
	} {

		if err := reader.Decode(iface); err != nil {
			return nil, err
		}
	}

	h.Logger.Info("creating m3dbcluster", zap.String("m3dbcluster", cluster.Name))
	cluster, err = h.CRDClient.OperatorV1alpha1().M3DBClusters(h.Namespace).
		Create(ctx, cluster, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return cluster, nil
}
