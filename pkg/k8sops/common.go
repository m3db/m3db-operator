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

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	headlessServicePrefix    = "m3dbnode-"
	coordinatorServicePrefix = "m3coordinator-"
)

// StatefulSetName provides a formatted string to use for naming StatefulSets
func StatefulSetName(clusterName string, stsID int) string {
	return fmt.Sprintf("%s-rep%d", clusterName, stsID)
}

// HeadlessServiceName returns a name for the cluster's headless service.
func HeadlessServiceName(clusterName string) string {
	return headlessServicePrefix + clusterName
}

// CoordinatorServiceName returns a name for a cluster's coordinator service.
func CoordinatorServiceName(clusterName string) string {
	return coordinatorServicePrefix + clusterName
}

// TODO(schallert): should figure out a better way to abstract this other than
// exposing all of CoreV1()
func (k *k8sops) Events(namespace string) typedcorev1.EventInterface {
	return k.kclient.CoreV1().Events(namespace)
}
