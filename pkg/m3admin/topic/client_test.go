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

package topic

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3/src/msg/generated/proto/topicpb"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTopicName = "someTestTopic"
)

func checkTopicNameHeader(t *testing.T, r *http.Request) {
	if r.Header.Get(headerTopicName) != testTopicName {
		t.Error("expected topic name header")
	}
}

// Client to avoid waiting many seconds in tests.
func newM3adminClient() m3admin.Client {
	retry := retryhttp.NewClient()
	retry.RetryMax = 0

	return m3admin.NewClient(m3admin.WithHTTPClient(retry))
}

func newTopicClient(t *testing.T, url string) Client {
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
		checkTopicNameHeader(t, r)
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newTopicClient(t, s.URL)

	err := client.Delete(testTopicName)
	require.Nil(t, err)
}

func TestDeleteErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newTopicClient(t, s.URL)

	err := client.Delete(testTopicName)
	require.NotNil(t, err)
}

func TestAdd(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}

		const expected = `{"consumerService":{"serviceId":{"name":"foo","environment":"bar"}}}`
		assert.Equal(t, expected, string(bytes))

		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newTopicClient(t, s.URL)

	err := client.Add(testTopicName, &topicpb.ConsumerService{
		ServiceId: &topicpb.ServiceID{
			Name:        "foo",
			Environment: "bar",
		},
	})
	require.Nil(t, err)
}

func TestAddErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newTopicClient(t, s.URL)

	err := client.Add(testTopicName, &topicpb.ConsumerService{
		ServiceId: &topicpb.ServiceID{
			Name:        "foo",
			Environment: "bar",
		},
	})
	require.NotNil(t, err)
}

func TestInit(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newTopicClient(t, s.URL)

	err := client.Init(testTopicName, &admin.TopicInitRequest{
		NumberOfShards: 64,
	})
	require.Nil(t, err)
}

func TestInitErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		w.WriteHeader(500)
		w.Write([]byte("{}"))
	}))

	defer s.Close()
	client := newTopicClient(t, s.URL)

	err := client.Init(testTopicName, &admin.TopicInitRequest{
		NumberOfShards: 64,
	})
	require.NotNil(t, err)
}

func TestGet(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		w.WriteHeader(200)
		w.Write([]byte(`{"topic": {}}`))
	}))
	defer s.Close()
	client := newTopicClient(t, s.URL)

	topic, err := client.Get(testTopicName)
	require.NotNil(t, topic)
	require.NoError(t, err)
}

func TestGetErr(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkTopicNameHeader(t, r)
		w.WriteHeader(404)
		w.Write([]byte("{}"))
	}))
	defer s.Close()

	client := newTopicClient(t, s.URL)

	topic, err := client.Get(testTopicName)
	require.Nil(t, topic)
	require.Error(t, err)
}
