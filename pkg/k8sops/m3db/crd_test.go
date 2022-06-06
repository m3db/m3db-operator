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

package m3db

import (
	"context"
	"errors"
	"testing"

	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	clientsetFake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kubeExtFake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrUpdateCRD(t *testing.T) {
	k := newFakeK8sops(t).(*k8sWrapper)
	ctx := context.Background()

	err := k.CreateOrUpdateCRD(ctx, "foo", false)
	assert.Error(t, err)

	ext := k.kubeExt.ApiextensionsV1().CustomResourceDefinitions()
	// Ensure CRD didn't exist before but exists after
	_, err = ext.Get(ctx, m3dboperator.M3DBClustersName, metav1.GetOptions{})
	assert.Error(t, err)

	// Create the CRD.
	err = k.CreateOrUpdateCRD(ctx, m3dboperator.M3DBClustersName, false)
	assert.NoError(t, err)

	_, err = ext.Get(ctx, m3dboperator.M3DBClustersName, metav1.GetOptions{})
	assert.NoError(t, err)

	// Update the CRD.
	err = k.CreateOrUpdateCRD(ctx, m3dboperator.M3DBClustersName, false)
	assert.NoError(t, err)
}

func TestCreateOrUpdateCRD_Err(t *testing.T) {
	for _, test := range []struct {
		action string
		expErr string
	}{
		{
			action: "get",
			expErr: "could not fetch",
		},
		{
			action: "update",
			expErr: "error updating",
		},
		{
			action: "create",
			expErr: "error creating",
		},
	} {
		t.Run(test.action, func(t *testing.T) {
			k := newFakeK8sops(t).(*k8sWrapper)
			ctx := context.Background()

			k.kubeExt.(*kubeExtFake.Clientset).Fake.PrependReactor(test.action, "*", func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, &extv1.CustomResourceDefinition{}, errors.New("test")
			})

			// Must create CRD first to have an error updating it.
			if test.action == "update" {
				err := k.CreateOrUpdateCRD(ctx, m3dboperator.M3DBClustersName, false)
				assert.NoError(t, err)
			}

			err := k.CreateOrUpdateCRD(ctx, m3dboperator.M3DBClustersName, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expErr)
		})
	}
}

func TestWaitForCRDReady(t *testing.T) {
	k := newFakeK8sops(t).(*k8sWrapper)
	ctx := context.Background()

	err := k.waitForCRDReady(ctx, "foo")
	assert.Error(t, err)

	err = k.waitForCRDReady(ctx, m3dboperator.M3DBClustersName)
	assert.NoError(t, err)

	k.crdClient.(*clientsetFake.Clientset).Fake.PrependReactor("list", "m3dbclusters", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, &myspec.M3DBClusterList{}, errors.New("test")
	})

	err = k.waitForCRDReady(ctx, m3dboperator.M3DBClustersName)
	assert.Error(t, err)
}
