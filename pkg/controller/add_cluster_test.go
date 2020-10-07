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
	"archive/zip"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rakyll/statik/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"
)

func registerValidConfigMap(content string) error {
	sw := &strings.Builder{}
	zw := zip.NewWriter(sw)

	// Build a zip fs containing our test config map
	fw, err := zw.Create("default-config.tmpl")
	if err != nil {
		return err
	}
	_, err = fw.Write([]byte(content))
	if err != nil {
		return err
	}
	err = zw.Close()
	if err != nil {
		return err
	}

	fs.Register(sw.String())
	return nil
}

func TestEnsureService_Base(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	k8sops := newFakeK8sops(t)

	c := &M3DBController{
		k8sclient: k8sops,
	}

	err := c.ensureServices(cluster)
	assert.NoError(t, err)

	for _, svcName := range []string{"m3dbnode-cluster-simple", "m3coordinator-cluster-simple"} {
		svc, err := k8sops.GetService(cluster, svcName)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	}
}

func TestEnsureConfigMap(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	deps := newTestDeps(t, &testOpts{})
	controller := deps.newController(t)
	defer deps.cleanup()

	require.NoError(t, registerValidConfigMap("my_config_data"))

	err := controller.ensureConfigMap(cluster)
	assert.NoError(t, err)

	cms, err := controller.kubeClient.CoreV1().ConfigMaps(cluster.Namespace).List(metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cluster.Name, cms.Items[0].OwnerReferences[0].Name)

	err = controller.ensureConfigMap(cluster)
	assert.NoError(t, err)

	cluster.Spec.ConfigMapName = pointer.StringPtr("")
	err = controller.ensureConfigMap(cluster)
	assert.Equal(t, errEmptyConfigMap, err)
}

func TestEnsureConfigMap_Update(t *testing.T) {
	cluster := getFixture("cluster-simple.yaml", t)
	deps := newTestDeps(t, &testOpts{})
	controller := deps.newController(t)
	defer deps.cleanup()

	require.NoError(t, registerValidConfigMap("my_config_data"))

	err := controller.ensureConfigMap(cluster)
	assert.NoError(t, err)

	// Change configmap data, expect to see changes reflected.
	require.NoError(t, registerValidConfigMap("new_config_data"))

	err = controller.ensureConfigMap(cluster)
	assert.NoError(t, err)

	cm, err := deps.kubeClient.CoreV1().ConfigMaps(cluster.Namespace).Get("m3db-config-map-cluster-simple", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "new_config_data", cm.Data["m3.yml"])
}
