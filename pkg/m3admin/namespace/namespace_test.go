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

package namespace

import (
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	m3ns "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	"github.com/stretchr/testify/assert"
)

func TestRequestFromSpec(t *testing.T) {
	tests := []struct {
		ns     myspec.Namespace
		req    *admin.NamespaceAddRequest
		expErr bool
	}{
		{
			ns:     myspec.Namespace{},
			expErr: true,
		},
		{
			ns: myspec.Namespace{
				Name: "foo",
			},
			expErr: true,
		},
		{
			ns: myspec.Namespace{
				Name:    "foo",
				Preset:  "a",
				Options: &myspec.NamespaceOptions{},
			},
			expErr: true,
		},
		{
			ns: myspec.Namespace{
				Name: "foo",
				Options: &myspec.NamespaceOptions{
					BootstrapEnabled: true,
					RetentionOptions: myspec.RetentionOptions{
						BlockDataExpiry: true,
					},
					IndexOptions: myspec.IndexOptions{
						Enabled: true,
					},
				},
			},
			req: &admin.NamespaceAddRequest{
				Name: "foo",
				Options: &m3ns.NamespaceOptions{
					BootstrapEnabled: true,
					RetentionOptions: &m3ns.RetentionOptions{
						BlockDataExpiry: true,
					},
					IndexOptions: &m3ns.IndexOptions{
						Enabled: true,
					},
				},
			},
		},
		{
			ns: myspec.Namespace{
				Name:   "foo",
				Preset: "a",
			},
			expErr: true,
		},
		{
			ns: myspec.Namespace{
				Name:   "foo",
				Preset: "10s:2d",
			},
			req: &admin.NamespaceAddRequest{
				Name:    "foo",
				Options: requestOptsFromAPI(&presetTenSecondsTwoDaysIndexed),
			},
		},
		{
			ns: myspec.Namespace{
				Name:   "foo",
				Preset: "1m:40d",
			},
			req: &admin.NamespaceAddRequest{
				Name:    "foo",
				Options: requestOptsFromAPI(&presetOneMinuteFourtyDaysIndexed),
			},
		},
	}

	for _, test := range tests {
		req, err := RequestFromSpec(test.ns)
		if test.expErr {
			assert.Error(t, err)
		} else {
			assert.Equal(t, test.req, req)
		}
	}
}

func TestRetentionOptsFromAPI(t *testing.T) {
	opts := myspec.RetentionOptions{
		RetentionPeriod:                     time.Second,
		BlockSize:                           2 * time.Second,
		BufferFuture:                        3 * time.Second,
		BufferPast:                          4 * time.Second,
		BlockDataExpiry:                     true,
		BlockDataExpiryAfterNotAccessPeriod: 5 * time.Second,
	}

	nsOpts := retentionOptsFromAPI(opts)

	assert.Equal(t, int64(1000000000), nsOpts.RetentionPeriodNanos)
	assert.Equal(t, int64(2000000000), nsOpts.BlockSizeNanos)
	assert.Equal(t, int64(3000000000), nsOpts.BufferFutureNanos)
	assert.Equal(t, int64(4000000000), nsOpts.BufferPastNanos)
	assert.True(t, nsOpts.BlockDataExpiry)
	assert.Equal(t, int64(5000000000), nsOpts.BlockDataExpiryAfterNotAccessPeriodNanos)
}

func TestIndexOptsFromAPI(t *testing.T) {
	opts := myspec.IndexOptions{
		Enabled:   true,
		BlockSize: time.Second,
	}

	iOpts := indexOptsFromAPI(opts)

	assert.True(t, iOpts.Enabled)
	assert.Equal(t, int64(1000000000), iOpts.BlockSizeNanos)
}
