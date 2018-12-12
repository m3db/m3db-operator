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

package v1alpha1

// PodIdentitySource indicates a pre-defined source for deriving pod identity.
type PodIdentitySource string

const (
	// PodIdentitySourcePodUID derives identity from UID of the pod.
	PodIdentitySourcePodUID PodIdentitySource = "PodUID"

	// PodIdentitySourceNodeSpecExternalID derives identity from the 'externalID'
	// spec field of the node. Note this field was deprecated after Kubernetes
	// 1.11.
	PodIdentitySourceNodeSpecExternalID PodIdentitySource = "NodeSpecExternalID"

	// PodIdentitySourceNodeSpecProviderID derives identity from the 'providerID'
	// spec field of the node.
	PodIdentitySourceNodeSpecProviderID PodIdentitySource = "NodeSpecProviderID"

	// PodIdentitySourceNodeName derives identity from the node's name.
	PodIdentitySourceNodeName PodIdentitySource = "NodeName"
)

// PodIdentity contains all the fields that may be used to identify a pod's
// identity in the M3DB placement. Any non-empty fields will be used to identity
// uniqueness of a pod for the purpose of M3DB replace operations.
type PodIdentity struct {
	Name           string `json:"name,omitempty"`
	UID            string `json:"uid,omitempty"`
	NodeName       string `json:"nodeName,omitempty"`
	NodeExternalID string `json:"nodeExternalID,omitempty"`
	NodeProviderID string `json:"nodeProviderID,omitempty"`
}

// PodIdentityConfig contains cluster-level configuration for deriving pod
// identity.
type PodIdentityConfig struct {
	// Sources enumerates the sources from which to derive pod identity. Note that
	// a pod's name will always be used. If empty, defaults to pod name and
	// UID.
	Sources []PodIdentitySource `json:"sources"`
}
