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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	scheme "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// M3DBClustersGetter has a method to return a M3DBClusterInterface.
// A group's client should implement this interface.
type M3DBClustersGetter interface {
	M3DBClusters(namespace string) M3DBClusterInterface
}

// M3DBClusterInterface has methods to work with M3DBCluster resources.
type M3DBClusterInterface interface {
	Create(*v1.M3DBCluster) (*v1.M3DBCluster, error)
	Update(*v1.M3DBCluster) (*v1.M3DBCluster, error)
	UpdateStatus(*v1.M3DBCluster) (*v1.M3DBCluster, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.M3DBCluster, error)
	List(opts metav1.ListOptions) (*v1.M3DBClusterList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.M3DBCluster, err error)
	M3DBClusterExpansion
}

// m3DBClusters implements M3DBClusterInterface
type m3DBClusters struct {
	client rest.Interface
	ns     string
}

// newM3DBClusters returns a M3DBClusters
func newM3DBClusters(c *OperatorV1Client, namespace string) *m3DBClusters {
	return &m3DBClusters{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the m3DBCluster, and returns the corresponding m3DBCluster object, and an error if there is any.
func (c *m3DBClusters) Get(name string, options metav1.GetOptions) (result *v1.M3DBCluster, err error) {
	result = &v1.M3DBCluster{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("m3dbclusters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of M3DBClusters that match those selectors.
func (c *m3DBClusters) List(opts metav1.ListOptions) (result *v1.M3DBClusterList, err error) {
	result = &v1.M3DBClusterList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("m3dbclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested m3DBClusters.
func (c *m3DBClusters) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("m3dbclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a m3DBCluster and creates it.  Returns the server's representation of the m3DBCluster, and an error, if there is any.
func (c *m3DBClusters) Create(m3DBCluster *v1.M3DBCluster) (result *v1.M3DBCluster, err error) {
	result = &v1.M3DBCluster{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("m3dbclusters").
		Body(m3DBCluster).
		Do().
		Into(result)
	return
}

// Update takes the representation of a m3DBCluster and updates it. Returns the server's representation of the m3DBCluster, and an error, if there is any.
func (c *m3DBClusters) Update(m3DBCluster *v1.M3DBCluster) (result *v1.M3DBCluster, err error) {
	result = &v1.M3DBCluster{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("m3dbclusters").
		Name(m3DBCluster.Name).
		Body(m3DBCluster).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *m3DBClusters) UpdateStatus(m3DBCluster *v1.M3DBCluster) (result *v1.M3DBCluster, err error) {
	result = &v1.M3DBCluster{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("m3dbclusters").
		Name(m3DBCluster.Name).
		SubResource("status").
		Body(m3DBCluster).
		Do().
		Into(result)
	return
}

// Delete takes name of the m3DBCluster and deletes it. Returns an error if one occurs.
func (c *m3DBClusters) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("m3dbclusters").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *m3DBClusters) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("m3dbclusters").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched m3DBCluster.
func (c *m3DBClusters) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.M3DBCluster, err error) {
	result = &v1.M3DBCluster{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("m3dbclusters").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
