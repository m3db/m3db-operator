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
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func devNullRetry() *retryhttp.Client {
	retry := retryhttp.NewClient()
	retry.Logger = zap.NewStdLog(zap.NewNop())
	return retry
}

func newTestClient() Client {
	return NewClient(WithHTTPClient(devNullRetry()))
}

func TestNewClient(t *testing.T) {
	cl := NewClient()
	assert.NotNil(t, cl)
}

func TestClient_DoHTTPRequest(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer s.Close()

	readAll := func(r io.Reader) []byte {
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		return data
	}

	cl := newTestClient()
	resp, err := cl.DoHTTPRequest("GET", s.URL, nil)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, []byte("hello"), readAll(resp.Body))

	// enable debug and make sure nothing fails
	l, err := zap.NewDevelopment()
	require.NoError(t, err)

	cl = NewClient(WithLogger(l), WithHTTPClient(devNullRetry()))
	resp, err = cl.DoHTTPRequest("GET", s.URL, nil)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, []byte("hello"), readAll(resp.Body))
}

func TestClient_DoHTTPRequest_Err(t *testing.T) {
	for _, test := range []struct {
		code   int
		expErr error
	}{
		{
			code:   404,
			expErr: ErrNotFound,
		},
		{
			code:   500,
			expErr: ErrNotOk,
		},
	} {
		t.Run(strconv.Itoa(test.code), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.code)
			}))
			defer s.Close()

			retry := devNullRetry()
			retry.RetryMax = 0
			retry.ErrorHandler = retryhttp.ErrorHandler(func(res *http.Response, err error, _ int) (*http.Response, error) {
				return res, err
			})

			cl := NewClient(WithHTTPClient(retry))
			_, err := cl.DoHTTPRequest("GET", s.URL, nil)
			assert.Equal(t, test.expErr, err)
		})
	}
}
