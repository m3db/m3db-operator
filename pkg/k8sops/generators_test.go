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
	"fmt"
	"io/ioutil"
	"testing"

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getFixture(filename string, t *testing.T) myspec.ClusterSpec {
	spec := myspec.ClusterSpec{}
	file, err := ioutil.ReadFile(fmt.Sprintf("./fixtures/%s", filename))
	if err != nil {
		t.Logf("Failed to read fixtures file: %v ", err)
		t.Fail()
	}
	err = yaml.Unmarshal(file, &spec)
	if err != nil {
		t.Logf("Unmarshal error: %v", err)
		t.Fail()
	}
	return spec
}

func TestGenerateCRD(t *testing.T) {
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: m3dboperator.Name,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   m3dboperator.GroupName,
			Version: m3dboperator.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: m3dboperator.ResourcePlural,
				Kind:   m3dboperator.ResourceKind,
			},
		},
	}

	k := newFakeK8sops()
	newCRD := k.GenerateCRD()
	require.Equal(t, crd, newCRD)
}

func TestGenerateService(t *testing.T) {
	fixture := getFixture("testCluster.yaml", t)
	svcCfg := fixture.ServiceConfigurations[0]
	k := newFakeK8sops()
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   svcCfg.Name,
			Labels: k.GenerateMaps("labels", svcCfg),
		},
		Spec: v1.ServiceSpec{
			Selector:  k.GenerateMaps("selectors", svcCfg),
			Ports:     k.GenerateServicePorts(svcCfg.Ports),
			ClusterIP: "None",
		},
	}
	newSvc := k.GenerateService(svcCfg)
	require.Equal(t, svc, newSvc)
}
