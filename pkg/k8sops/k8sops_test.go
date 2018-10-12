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
	"os"
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	clientsetFake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"

	appsv1 "k8s.io/api/apps/v1"
	kubeExtFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	kubeFake "k8s.io/client-go/kubernetes/fake"

	"go.uber.org/zap"
)

func newFakeK8sops() (K8sops, error) {
	logger := zap.NewNop()
	kubeCli := kubeFake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "m3db-cluster-rep0",
		},
	})
	kubeExt := kubeExtFake.NewSimpleClientset()
	crdCli := clientsetFake.NewSimpleClientset()
	k, err := New(
		WithCRDClient(crdCli),
		WithExtClient(kubeExt),
		WithKClient(kubeCli),
		WithLogger(logger),
	)
	return k, err
}

func getFixture(filename string, t *testing.T) *myspec.M3DBCluster {
	spec := &myspec.M3DBCluster{}
	file, err := os.Open(fmt.Sprintf("./fixtures/%s", filename))
	if err != nil {
		t.Logf("Failed to read fixtures file: %v ", err)
		t.Fail()
	}
	err = yaml.NewYAMLOrJSONDecoder(file, 2048).Decode(&spec)
	if err != nil {
		t.Logf("Unmarshal error: %v", err)
		t.Fail()
	}
	return spec
}
