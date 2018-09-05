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

package m3admin

import (
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"
)

const (
	DefaultServicePort   = 7201
	DefaultServiceName   = "m3coordinator"
	DefaultServiceDomain = "default"
)

type Namespace interface {
	// Create will create a namepace with provided namespace name and defaults
	Create(namespace string) error
	// Get will retrieve a namespace given a name
	Get() (*admin.NamespaceGetResponse, error)
	// Delete will delete a namespace given a name
	Delete(namespace string) error
}

type Placement interface {
	// Init will initialize a placement give a valid placement request
	Init(request *admin.PlacementInitRequest) error
	// Get will provide a placement given a hostname e.g. a subdomin
	Get() (*admin.PlacementGetResponse, error)
	// Delete will delete a placment given a hostname e.g. a subdomain
	Delete() error
	// Add will add a placement given a hostname e.g. a subdomain
	Add(instance placementpb.Instance) (*admin.PlacementGetResponse, error)
}
