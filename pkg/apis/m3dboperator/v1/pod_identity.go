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

package v1

// PodIdentitySource indicates a pre-defined source for deriving pod identity.
type PodIdentitySource string

const (
	// PodIdentitySourcePodUID derives identity from UID of the pod.
	PodIdentitySourcePodUID PodIdentitySource = "PodUID"
	// PodIdentitySourcePodName derives identity from the name of the pod.
	PodIdentitySourcePodName PodIdentitySource = "PodName"
)

// PodIdentity contains all the fields that may be used to identify a pod's
// identity in the M3DB placement. Any non-empty fields will be used to identity
// uniqueness of a pod for the purpose of M3DB replace operations.
type PodIdentity struct {
	Name string `json:"name,omitempty"`
	UID  string `json:"uid,omitempty"`
}

// PodIdentityConfig contains cluster-level configuration for deriving pod
// identity.
type PodIdentityConfig struct {
	Sources []PodIdentitySource `json:"sources"`
}
