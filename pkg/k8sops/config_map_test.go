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
	"archive/zip"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/kubernetes/utils/pointer"
	"github.com/rakyll/statik/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerValidConfigMap() error {
	sw := &strings.Builder{}
	zw := zip.NewWriter(sw)

	// Build a zip fs containing our test config map
	fw, err := zw.Create("default-config.yaml")
	if err != nil {
		return err
	}
	_, err = fw.Write([]byte("my_config_data"))
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

func TestGenerateDefaultConfigMap(t *testing.T) {
	cluster := getFixture("testM3DBCluster.yaml", t)

	require.NoError(t, registerValidConfigMap())

	cm, err := GenerateDefaultConfigMap(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	assert.Equal(t, "my_config_data", cm.Data["m3.yml"])
	assert.Equal(t, "m3db-config-map-m3db-cluster", cm.Name)
	assert.Equal(t, "m3db-cluster", cm.OwnerReferences[0].Name)

	// Build a zip FS without our default config map and ensure error.
	sw := &strings.Builder{}
	zw := zip.NewWriter(sw)
	fw, err := zw.Create("foopath.yaml")
	require.NoError(t, err)
	_, err = fw.Write([]byte("my_config_data"))
	require.NoError(t, err)
	err = zw.Close()
	require.NoError(t, err)

	fs.Register(sw.String())
	_, err = GenerateDefaultConfigMap(cluster)
	require.Error(t, err)
	assert.Equal(t, "file does not exist", err.Error())

	// Register garbage zip data and ensure error
	fs.Register("foo")
	_, err = GenerateDefaultConfigMap(cluster)
	assert.Error(t, err)

	cluster.Spec.ConfigMapName = pointer.StringPtr("foo")
	_, err = GenerateDefaultConfigMap(cluster)
	assert.Equal(t, errConfigMapNonNil, err)
}

func TestBuildConfigMapComponents(t *testing.T) {
	cluster := getFixture("testM3DBCluster.yaml", t)

	vol, vm, err := buildConfigMapComponents(cluster)
	assert.NoError(t, err)

	expVM := corev1.VolumeMount{
		Name:      "m3-configuration",
		MountPath: "/etc/m3db/",
	}

	assert.Equal(t, expVM, vm)
	assert.Equal(t, "m3-configuration", vol.Name)
	assert.Equal(t, "m3db-config-map-m3db-cluster", vol.VolumeSource.ConfigMap.Name)

	cluster.Spec.ConfigMapName = pointer.StringPtr("foo")
	vol, vm, err = buildConfigMapComponents(cluster)
	assert.NoError(t, err)
	assert.Equal(t, expVM, vm)
	assert.Equal(t, "m3-configuration", vol.Name)
	assert.Equal(t, "foo", vol.VolumeSource.ConfigMap.Name)

	cluster.Spec.ConfigMapName = pointer.StringPtr("")
	_, _, err = buildConfigMapComponents(cluster)
	assert.Equal(t, errEmptyConfigMapName, err)
}
