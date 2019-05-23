// Copyright (c) 2019 Uber Technologies, Inc.
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
	"errors"
	"testing"

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	clientsetFake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kubeExtFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrUpdateCRD(t *testing.T) {
	k := newFakeK8sops(t).(*k8sops)

	err := k.CreateOrUpdateCRD("foo", false)
	assert.Error(t, err)

	ext := k.kubeExt.ApiextensionsV1beta1().CustomResourceDefinitions()
	// Ensure CRD didn't exist before but exists after
	_, err = ext.Get(m3dboperator.Name, metav1.GetOptions{})
	assert.Error(t, err)

	err = k.CreateOrUpdateCRD(m3dboperator.Name, false)
	assert.NoError(t, err)

	_, err = ext.Get(m3dboperator.Name, metav1.GetOptions{})
	assert.NoError(t, err)
}

func TestCreateOrUpdateCRD_ErrGet(t *testing.T) {
	k := newFakeK8sops(t).(*k8sops)

	err := k.CreateOrUpdateCRD("foo", false)
	assert.Error(t, err)

	k.kubeExt.(*kubeExtFake.Clientset).Fake.PrependReactor("get", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &extv1beta1.CustomResourceDefinition{}, errors.New("test")
	})
	err = k.CreateOrUpdateCRD(m3dboperator.Name, false)
	assert.Contains(t, err.Error(), "could not fetch")
}

func TestCreateOrUpdateCRD_ErrUpdate(t *testing.T) {
	k := newFakeK8sops(t).(*k8sops)

	err := k.CreateOrUpdateCRD("foo", false)
	assert.Error(t, err)

	err = k.CreateOrUpdateCRD(m3dboperator.Name, false)
	assert.NoError(t, err)

	k.kubeExt.(*kubeExtFake.Clientset).Fake.PrependReactor("update", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &extv1beta1.CustomResourceDefinition{}, errors.New("test")
	})
	err = k.CreateOrUpdateCRD(m3dboperator.Name, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error updating")
}

func TestWaitForCRDReady(t *testing.T) {
	k := newFakeK8sops(t).(*k8sops)

	err := k.waitForCRDReady("foo")
	assert.Error(t, err)

	err = k.waitForCRDReady(m3dboperator.Name)
	assert.NoError(t, err)

	k.crdClient.(*clientsetFake.Clientset).Fake.PrependReactor("list", "m3dbclusters", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &myspec.M3DBClusterList{}, errors.New("test")
	})

	err = k.waitForCRDReady(m3dboperator.Name)
	assert.Error(t, err)
}
