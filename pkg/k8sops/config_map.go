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
	"bytes"
	"errors"
	"text/template"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rakyll/statik/fs"
)

const (
	defaultConfigMapTemplateAssetPath = "/default-config.tmpl"
)

var (
	errConfigMapNonNil    = errors.New("cannot generate configmap when cluster specified one")
	errEmptyConfigMapName = errors.New("configMap name cannot be empty if non-nil")
	errEmptyEtcdEndpoits  = errors.New("etcd endpoints cannot be empty with default configmap")
)

type configData struct {
	Env       string
	Endpoints []string
	CarbonIngester bool
}

// GenerateDefaultConfigMap creates a ConfigMap for the clusters with the
// default config.
func GenerateDefaultConfigMap(cluster *myspec.M3DBCluster) (*corev1.ConfigMap, error) {
	if cluster.Spec.ConfigMapName != nil {
		return nil, errConfigMapNonNil
	}

	if len(cluster.Spec.EtcdEndpoints) == 0 {
		return nil, errEmptyEtcdEndpoits
	}

	hfs, err := fs.New()
	if err != nil {
		return nil, err
	}

	templateData, err := fs.ReadFile(hfs, defaultConfigMapTemplateAssetPath)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("config").Parse(string(templateData))
	if err != nil {
		return nil, err
	}

	config := &configData{
		Env:       DefaultM3ClusterEnvironmentName(cluster),
		Endpoints: cluster.Spec.EtcdEndpoints,
		CarbonIngester: cluster.Spec.EnableCarbonIngester,
	}

	buf := &bytes.Buffer{}
	if err:= tmpl.Execute(buf, &config); err != nil {
		return nil, err
	}

	ownerRef := GenerateOwnerRef(cluster)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            defaultConfigMapName(cluster.Name),
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
		Data: map[string]string{
			_configurationFileName: buf.String(),
		},
	}

	return cm, nil
}

func defaultConfigMapName(clusterName string) string {
	return "m3db-config-map-" + clusterName
}

// DefaultM3ClusterEnvironmentName returns the environment under which cluster
// topology and runtime configuration will be stored. This ensures that multiple
// m3db clusters won't conflict with each other when sharing a backing etcd
// store.
func DefaultM3ClusterEnvironmentName(cluster *myspec.M3DBCluster) string {
	return cluster.Namespace + "/" + cluster.Name
}

// Build the volume for the pod and the volumeMount for the container containing
// necessary config map info. If a user specified a configMap of their own we'll
// mount it, otherwise we mount the default one (the controller is expected to
// ensure it exists if so.).
func buildConfigMapComponents(cluster *myspec.M3DBCluster) (corev1.Volume, corev1.VolumeMount, error) {
	vm := corev1.VolumeMount{
		Name:      _configurationName,
		MountPath: _configurationDirectory,
	}

	cmName := defaultConfigMapName(cluster.Name)
	if cluster.Spec.ConfigMapName != nil {
		cmName = *cluster.Spec.ConfigMapName
	}

	if cmName == "" {
		return corev1.Volume{}, corev1.VolumeMount{}, errEmptyConfigMapName
	}

	vol := corev1.Volume{
		Name: _configurationName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmName,
				},
			},
		},
	}

	return vol, vm, nil
}
