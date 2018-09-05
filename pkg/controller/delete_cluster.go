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
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
)

func (c *Controller) deleteM3DBCluster(cluster *myspec.M3DBCluster) error {
	if err := c.namespaceClient.Delete(cluster.GetObjectMeta().GetName()); err != nil {
		return err
	}
	if err := c.placementClient.Delete(); err != nil {
		return err
	}
	if err := c.k8sclient.DeleteStatefuleSets(cluster, c.k8sclient.LabelSelector("cluster", cluster.GetName())); err != nil {
		return err
	}
	if err := c.k8sclient.DeleteService(cluster, "m3dbnode"); err != nil {
		return err
	}
	if err := c.k8sclient.DeleteService(cluster, "m3coordinator"); err != nil {
		return err
	}

	return nil
}
