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

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	m3ns "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestFromSpec(t *testing.T) {
	preset10s2d, err := m3dbNamespaceOptsFromSpec(&presetTenSecondsTwoDaysIndexed)
	require.NoError(t, err)
	preset1m40d, err := m3dbNamespaceOptsFromSpec(&presetOneMinuteFourtyDaysIndexed)
	require.NoError(t, err)

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
				Name: "empty",
			},
			expErr: true,
		},
		{
			ns: myspec.Namespace{
				Name:    "badpreset",
				Preset:  "a",
				Options: &myspec.NamespaceOptions{},
			},
			expErr: true,
		},
		{
			ns: myspec.Namespace{
				Name: "validcustom",
				Options: &myspec.NamespaceOptions{
					BootstrapEnabled: true,
					RetentionOptions: myspec.RetentionOptions{
						RetentionPeriod:                     "1s",
						BlockSize:                           "1s",
						BufferFuture:                        "1s",
						BufferPast:                          "1s",
						BlockDataExpiry:                     true,
						BlockDataExpiryAfterNotAccessPeriod: "1s",
					},
					IndexOptions: myspec.IndexOptions{
						BlockSize: "1s",
						Enabled:   true,
					},
				},
			},
			req: &admin.NamespaceAddRequest{
				Name: "validcustom",
				Options: &m3ns.NamespaceOptions{
					BootstrapEnabled: true,
					RetentionOptions: &m3ns.RetentionOptions{
						RetentionPeriodNanos:                     1000000000,
						BlockSizeNanos:                           1000000000,
						BufferFutureNanos:                        1000000000,
						BufferPastNanos:                          1000000000,
						BlockDataExpiry:                          true,
						BlockDataExpiryAfterNotAccessPeriodNanos: 1000000000,
					},
					IndexOptions: &m3ns.IndexOptions{
						BlockSizeNanos: 1000000000,
						Enabled:        true,
					},
				},
			},
		},
		{
			ns: myspec.Namespace{
				Name: "invalidcustom",
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
				Name: "invalidcustom",
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
			expErr: true,
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
				Options: preset10s2d,
			},
		},
		{
			ns: myspec.Namespace{
				Name:   "foo",
				Preset: "1m:40d",
			},
			req: &admin.NamespaceAddRequest{
				Name:    "foo",
				Options: preset1m40d,
			},
		},
	}

	for _, test := range tests {
		req, err := RequestFromSpec(test.ns)
		if test.expErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.req, req)
		}
	}
}

func TestRetentionOptsFromAPI(t *testing.T) {
	opts := myspec.RetentionOptions{
		RetentionPeriod:                     time.Duration(time.Second).String(),
		BlockSize:                           time.Duration(2 * time.Second).String(),
		BufferFuture:                        time.Duration(3 * time.Second).String(),
		BufferPast:                          time.Duration(4 * time.Second).String(),
		BlockDataExpiry:                     true,
		BlockDataExpiryAfterNotAccessPeriod: time.Duration(5 * time.Second).String(),
	}

	nsOpts, err := m3dbRetentionOptsFromSpec(opts)
	assert.NoError(t, err)

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
		BlockSize: time.Second.String(),
	}

	iOpts, err := m3dbIndexOptsFromSpec(opts)
	assert.NoError(t, err)

	assert.True(t, iOpts.Enabled)
	assert.Equal(t, int64(1000000000), iOpts.BlockSizeNanos)
}
