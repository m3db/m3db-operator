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

package placement

import (
	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	m3placement "github.com/m3db/m3/src/cluster/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
)

// Client provides the interface to interact with the placement API
type Client interface {
	// Init will initialize a placement give a valid placement request
	Init(request *admin.PlacementInitRequest) error
	// Set will forcefully override a placement with a given value.
	Set(request *admin.PlacementSetRequest) error
	// Get will provide the current placement
	Get() (placement m3placement.Placement, err error)
	// Delete will delete the current placement
	Delete() error
	// Add will add an instance to the placement
	Add(instances []*placementpb.Instance) error
	// Remove removes instances with the given IDs from the placement.
	Remove(instanceIds []string) error
	// Replace replaces one instance with another.
	Replace(leavingInstanceID string, newInstance placementpb.Instance) error
}
