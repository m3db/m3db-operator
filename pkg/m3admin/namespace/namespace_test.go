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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/require"
)

// Client to avoid waiting many seconds in tests.
func newM3adminClient() m3admin.Client {
	retry := retryhttp.NewClient()
	retry.RetryMax = 0

	return m3admin.NewClient(m3admin.WithHTTPClient(retry))
}

func newNamespaceClient(t *testing.T, url string) Client {
	cl, err := NewClient(
		WithURL(url),
		WithClient(newM3adminClient()),
	)

	require.NoError(t, err)
	require.NotNil(t, cl)

	return cl
}

func TestCreate(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newNamespaceClient(t, s.URL)

	err := client.Create("test")
	require.Nil(t, err)
}

func TestCreateErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newNamespaceClient(t, s.URL)

	err := client.Create("test")
	require.NotNil(t, err)
}

func TestGet(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"registry":{"namespaces":{"default":{"bootstrapEnabled":true,"flushEnabled":true,"writesToCommitLog":true,"cleanupEnabled":true,"repairEnabled":false,"retentionOptions":{"retentionPeriodNanos":"172800000000000","blockSizeNanos":"7200000000000","bufferFutureNanos":"600000000000","bufferPastNanos":"600000000000","blockDataExpiry":true,"blockDataExpiryAfterNotAccessPeriodNanos":"300000000000"},"snapshotEnabled":false,"indexOptions":{"enabled":true,"blockSizeNanos":"7200000000000"}},"m3db-cluster":{"bootstrapEnabled":true,"flushEnabled":true,"writesToCommitLog":true,"cleanupEnabled":true,"repairEnabled":false,"retentionOptions":{"retentionPeriodNanos":"172800000000000","blockSizeNanos":"7200000000000","bufferFutureNanos":"600000000000","bufferPastNanos":"600000000000","blockDataExpiry":true,"blockDataExpiryAfterNotAccessPeriodNanos":"300000000000"},"snapshotEnabled":false,"indexOptions":{"enabled":true,"blockSizeNanos":"7200000000000"}}}}}`))
	}))
	defer s.Close()
	client := newNamespaceClient(t, s.URL)

	resp, err := client.List()
	require.NotNil(t, resp)
	require.Nil(t, err)
}

func TestGetErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))
	defer s.Close()
	client := newNamespaceClient(t, s.URL)

	resp, err := client.List()
	require.Nil(t, resp)
	require.NotNil(t, err)
}

func TestDelete(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer s.Close()
	client := newNamespaceClient(t, s.URL)

	err := client.Delete("default")
	require.Nil(t, err)
}

func TestDeleteErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))
	defer s.Close()
	client := newNamespaceClient(t, s.URL)

	err := client.Delete("default")
	require.NotNil(t, err)
}
