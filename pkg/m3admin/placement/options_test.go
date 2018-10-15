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
	"testing"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestOptions(t *testing.T) {
	client := &placementClient{}

	opt := WithURL("...")
	assert.Error(t, opt.execute(client))

	const url = "http://localhost:1234"
	l := zap.NewNop()
	m3cl := m3admin.NewClient()
	opts := []Option{
		WithURL(url),
		WithLogger(l),
		WithClient(m3cl),
	}

	for _, opt := range opts {
		assert.NoError(t, opt.execute(client))
	}

	assert.Equal(t, l, client.logger)
	assert.Equal(t, m3cl, client.client)
	assert.Equal(t, url, client.url)
}
