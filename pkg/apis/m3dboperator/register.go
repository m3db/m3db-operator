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

package m3dboperator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// ResourceKind is the custom resource kind
	ResourceKind = "M3DBCluster"

	// ResourcePlural and GroupName comprise the fully qualified DNS name
	// for the cluster. Naming must follow the convention stated below
	//
	// a DNS-1123 subdomain must consist of lower case alphanumeric characters,
	// '-' or '.', and must start and end with an alphanumeric character
	// (e.g. 'example.com', regex used for validation is
	// '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')

	// ResourcePlural is the plural form of custom resource kind
	ResourcePlural = "m3dbclusters"

	// GroupName is the group that the custom resource belongs to
	GroupName = "operator.m3db.io"

	// Version sets the version of the custom resource
	Version = "v1"
)

var (
	// Name is the fully qualified name of the custom resource
	Name = fmt.Sprintf("%s.%s", ResourcePlural, GroupName)

	// SchemeGroupVersion is the schema version of the group
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
)
