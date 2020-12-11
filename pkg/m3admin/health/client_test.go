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

package health

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/golang/mock/gomock"
	retryhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/require"
)

func TestBootstrapped(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newHealthClientWithURL(t, newM3adminClient(s.Client()), s.URL)

	bootstrapped, err := client.Bootstrapped("foo", "bar")

	require.NoError(t, err)
	require.True(t, bootstrapped)
}

func TestBootstrapped_ValidateURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adminClient := m3admin.NewMockClient(ctrl)
	adminClient.EXPECT().DoHTTPRequest("GET", "http://bar.m3dbnode-m3-cluster-short.foo/bootstrapped", nil).
		Return(&http.Response{}, nil)

	client := newHealthClient(t, adminClient)

	bootstrapped, err := client.Bootstrapped("foo", "bar")

	require.NoError(t, err)
	require.True(t, bootstrapped)
}

func TestBootstrapped_NotReady(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		w.WriteHeader(500)
		w.Write([]byte(notReadyJSON))
	}))

	defer s.Close()
	client := newHealthClientWithURL(t, newM3adminClient(s.Client()), s.URL)

	bootstrapped, err := client.Bootstrapped("foo", "bar")

	require.Error(t, err)
	require.False(t, bootstrapped)
}

const (
	notReadyJSON = "{\"error\":{\"message\":\"Error({Type:INTERNAL_ERROR Message:node is not bootstrapped}" +
		")\",\"data\":{\"type\":\"INTERNAL_ERROR\",\"message\":\"node is not bootstrapped\"}}}"
)

func newM3adminClient(client *http.Client) m3admin.Client {
	retry := retryhttp.NewClient()
	retry.HTTPClient = client
	retry.RetryMax = 0

	return m3admin.NewClient(m3admin.WithHTTPClient(retry))
}

func newHealthClient(t *testing.T, client m3admin.Client) Client {
	return newHealthClientWithURL(t, client, "")
}

func newHealthClientWithURL(t *testing.T, client m3admin.Client, url string) Client {
	cl, err := NewClient(
		WithClient(client),
		WithURL(url),
	)

	require.NoError(t, err)
	require.NotNil(t, cl)

	return cl
}
