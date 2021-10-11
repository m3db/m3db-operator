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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeM3DBClusters implements M3DBClusterInterface
type FakeM3DBClusters struct {
	Fake *FakeOperatorV1alpha1
	ns   string
}

var m3dbclustersResource = schema.GroupVersionResource{Group: "operator.m3db.io", Version: "v1alpha1", Resource: "m3dbclusters"}

var m3dbclustersKind = schema.GroupVersionKind{Group: "operator.m3db.io", Version: "v1alpha1", Kind: "M3DBCluster"}

// Get takes name of the m3DBCluster, and returns the corresponding m3DBCluster object, and an error if there is any.
func (c *FakeM3DBClusters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.M3DBCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(m3dbclustersResource, c.ns, name), &v1alpha1.M3DBCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.M3DBCluster), err
}

// List takes label and field selectors, and returns the list of M3DBClusters that match those selectors.
func (c *FakeM3DBClusters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.M3DBClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(m3dbclustersResource, m3dbclustersKind, c.ns, opts), &v1alpha1.M3DBClusterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.M3DBClusterList{ListMeta: obj.(*v1alpha1.M3DBClusterList).ListMeta}
	for _, item := range obj.(*v1alpha1.M3DBClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested m3DBClusters.
func (c *FakeM3DBClusters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(m3dbclustersResource, c.ns, opts))

}

// Create takes the representation of a m3DBCluster and creates it.  Returns the server's representation of the m3DBCluster, and an error, if there is any.
func (c *FakeM3DBClusters) Create(ctx context.Context, m3DBCluster *v1alpha1.M3DBCluster, opts v1.CreateOptions) (result *v1alpha1.M3DBCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(m3dbclustersResource, c.ns, m3DBCluster), &v1alpha1.M3DBCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.M3DBCluster), err
}

// Update takes the representation of a m3DBCluster and updates it. Returns the server's representation of the m3DBCluster, and an error, if there is any.
func (c *FakeM3DBClusters) Update(ctx context.Context, m3DBCluster *v1alpha1.M3DBCluster, opts v1.UpdateOptions) (result *v1alpha1.M3DBCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(m3dbclustersResource, c.ns, m3DBCluster), &v1alpha1.M3DBCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.M3DBCluster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeM3DBClusters) UpdateStatus(ctx context.Context, m3DBCluster *v1alpha1.M3DBCluster, opts v1.UpdateOptions) (*v1alpha1.M3DBCluster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(m3dbclustersResource, "status", c.ns, m3DBCluster), &v1alpha1.M3DBCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.M3DBCluster), err
}

// Delete takes name of the m3DBCluster and deletes it. Returns an error if one occurs.
func (c *FakeM3DBClusters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(m3dbclustersResource, c.ns, name), &v1alpha1.M3DBCluster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeM3DBClusters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(m3dbclustersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.M3DBClusterList{})
	return err
}

// Patch applies the patch and returns the patched m3DBCluster.
func (c *FakeM3DBClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.M3DBCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(m3dbclustersResource, c.ns, name, pt, data, subresources...), &v1alpha1.M3DBCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.M3DBCluster), err
}
