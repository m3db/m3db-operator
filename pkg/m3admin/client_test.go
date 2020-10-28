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
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	"github.com/gogo/protobuf/proto"
	retryhttp "github.com/hashicorp/go-retryablehttp"
	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func devNullRetry() *retryhttp.Client {
	retry := retryhttp.NewClient()
	retry.Logger = zap.NewStdLog(zap.NewNop())
	retry.RetryMax = 0
	return retry
}

func newTestClient(opts ...Option) Client {
	return NewClient(append([]Option{WithHTTPClient(devNullRetry())}, opts...)...)
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

func TestClient_DoHTTPRequest_Header(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(m3EnvironmentHeader) != "foo-env" {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte("hello"))
	}))
	defer s.Close()

	readAll := func(r io.Reader) []byte {
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		return data
	}

	cl := newTestClient(WithEnvironment("fooz-env"))
	_, err := cl.DoHTTPRequest("GET", s.URL, nil)
	assert.Error(t, err)

	cl = newTestClient(WithEnvironment("foo-env"))
	resp, err := cl.DoHTTPRequest("GET", s.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, []byte("hello"), readAll(resp.Body))
}

func TestClient_DoHTTPRequest_PerRequestHeader(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("test-header") != "test-val" {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte("hello"))
	}))
	defer s.Close()

	readAll := func(r io.Reader) []byte {
		data, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		return data
	}

	cl := newTestClient(WithHTTPClient(devNullRetry()))
	_, err := cl.DoHTTPRequest("GET", s.URL, nil)
	assert.Error(t, err)

	resp, err := cl.DoHTTPRequest("GET", s.URL, nil, WithHeader("test-header", "test-val"))
	require.NoError(t, err)
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
			code:   405,
			expErr: ErrMethodNotAllowed,
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
			assert.Equal(t, test.expErr, pkgerrors.Cause(err))
		})
	}
}

func TestClient_DoHTTPJSONPBRequest(t *testing.T) {
	marshal := func(m proto.Message) []byte {
		buf := bytes.NewBuffer(nil)
		require.NoError(t, JSONPBMarshal(buf, m))
		return buf.Bytes()
	}

	addJSONField := func(data []byte, key string, value interface{}) []byte {
		var r map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &r))
		r[key] = value
		data, err := json.Marshal(r)
		require.NoError(t, err)
		return data
	}

	for _, test := range []struct {
		name        string
		respBytes   []byte
		respMessage proto.Message
		expErr      bool
	}{
		{
			name: "unmarshal valid",
			respBytes: marshal(&admin.NamespaceGetResponse{
				Registry: &namespace.Registry{},
			}),
			respMessage: &admin.NamespaceGetResponse{},
		},
		{
			name: "unmarshal valid with unknown field",
			respBytes: addJSONField(marshal(&admin.NamespaceGetResponse{
				Registry: &namespace.Registry{},
			}), "foo", "bar"),
			respMessage: &admin.NamespaceGetResponse{},
		},
		{
			name:        "unmarshal invalid",
			respBytes:   []byte(`{"registry":"not_object"}`),
			respMessage: &admin.NamespaceGetResponse{},
			expErr:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Log("server sends", string(test.respBytes))
				w.Write(test.respBytes)
			}))
			defer s.Close()

			cl := NewClient()
			err := cl.DoHTTPJSONPBRequest(http.MethodGet, s.URL, nil, test.respMessage)
			if test.expErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
