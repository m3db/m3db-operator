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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// M3DBClusterLister helps list M3DBClusters.
type M3DBClusterLister interface {
	// List lists all M3DBClusters in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.M3DBCluster, err error)
	// M3DBClusters returns an object that can list and get M3DBClusters.
	M3DBClusters(namespace string) M3DBClusterNamespaceLister
	M3DBClusterListerExpansion
}

// m3DBClusterLister implements the M3DBClusterLister interface.
type m3DBClusterLister struct {
	indexer cache.Indexer
}

// NewM3DBClusterLister returns a new M3DBClusterLister.
func NewM3DBClusterLister(indexer cache.Indexer) M3DBClusterLister {
	return &m3DBClusterLister{indexer: indexer}
}

// List lists all M3DBClusters in the indexer.
func (s *m3DBClusterLister) List(selector labels.Selector) (ret []*v1alpha1.M3DBCluster, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.M3DBCluster))
	})
	return ret, err
}

// M3DBClusters returns an object that can list and get M3DBClusters.
func (s *m3DBClusterLister) M3DBClusters(namespace string) M3DBClusterNamespaceLister {
	return m3DBClusterNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// M3DBClusterNamespaceLister helps list and get M3DBClusters.
type M3DBClusterNamespaceLister interface {
	// List lists all M3DBClusters in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.M3DBCluster, err error)
	// Get retrieves the M3DBCluster from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.M3DBCluster, error)
	M3DBClusterNamespaceListerExpansion
}

// m3DBClusterNamespaceLister implements the M3DBClusterNamespaceLister
// interface.
type m3DBClusterNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all M3DBClusters in the indexer for a given namespace.
func (s m3DBClusterNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.M3DBCluster, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.M3DBCluster))
	})
	return ret, err
}

// Get retrieves the M3DBCluster from the indexer for a given namespace and name.
func (s m3DBClusterNamespaceLister) Get(name string) (*v1alpha1.M3DBCluster, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("m3dbcluster"), name)
	}
	return obj.(*v1alpha1.M3DBCluster), nil
}
