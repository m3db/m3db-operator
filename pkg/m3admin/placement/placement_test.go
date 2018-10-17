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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Client to avoid waiting many seconds in tests.
func newM3adminClient() m3admin.Client {
	retry := retryhttp.NewClient()
	retry.RetryMax = 0

	return m3admin.NewClient(m3admin.WithHTTPClient(retry))
}

func newPlacementClient(t *testing.T, url string) Client {
	cl, err := NewClient(
		WithURL(url),
		WithClient(newM3adminClient()),
	)

	require.NoError(t, err)
	require.NotNil(t, cl)

	return cl
}

func TestDelete(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newPlacementClient(t, s.URL)

	err := client.Delete()
	require.Nil(t, err)
}

func TestDeleteErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newPlacementClient(t, s.URL)

	err := client.Delete()
	require.NotNil(t, err)
}

func TestAdd(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}

		const expected = `{"instances":[{"id":"a"}]}`
		assert.Equal(t, expected, string(bytes))

		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newPlacementClient(t, s.URL)

	err := client.Add(placementpb.Instance{Id: "a"})
	require.Nil(t, err)
}

func TestAddErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newPlacementClient(t, s.URL)

	err := client.Add(placementpb.Instance{})
	require.NotNil(t, err)
}

func TestInit(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newPlacementClient(t, s.URL)

	err := client.Init(&admin.PlacementInitRequest{})
	require.Nil(t, err)
}

func TestInitErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newPlacementClient(t, s.URL)

	err := client.Init(&admin.PlacementInitRequest{})
	require.NotNil(t, err)
}

func TestGet(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"placement": {}}`))
	}))
	defer s.Close()
	client := newPlacementClient(t, s.URL)

	placement, err := client.Get()
	require.NotNil(t, placement)
	require.NoError(t, err)
}

func TestGetErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("{}"))
	}))
	defer s.Close()

	client := newPlacementClient(t, s.URL)

	placement, err := client.Get()
	require.Nil(t, placement)
	require.Error(t, err)
}

func TestRemove(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/api/v1/services/m3db/placement/instFoo" || r.Method != http.MethodDelete {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"placement": {}}`))
	}))
	defer s.Close()

	client := newPlacementClient(t, s.URL)
	err := client.Remove("instFoo")
	assert.NoError(t, err)
}
