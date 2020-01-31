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
	"errors"
	"net/http"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3/src/msg/generated/proto/topicpb"
	m3topic "github.com/m3db/m3/src/msg/topic"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	"go.uber.org/zap"
)

const (
	headerTopicName = "topic-name"

	topicBaseURL = "/api/v1/topic"
	topicInitURL = topicBaseURL + "/init"
	topicAddURL  = topicBaseURL
)

type topicClient struct {
	url    string
	client m3admin.Client
	logger *zap.Logger
}

// NewClient creates a new topic client.
func NewClient(opts ...Option) (Client, error) {
	logger := zap.NewNop()
	tc := &topicClient{
		client: m3admin.NewClient(),
		logger: logger,
	}

	for _, o := range opts {
		if err := o.execute(tc); err != nil {
			return nil, err
		}
	}
	return tc, nil
}

// Init creates a given topic.
func (t *topicClient) Init(name string, req *admin.TopicInitRequest) error {
	url := t.url + topicInitURL
	err := t.client.DoHTTPJSONPBRequest(http.MethodPost, url, req, nil, withTopicName(name))
	if err != nil {
		return err
	}
	t.logger.Info("successfully created topic")
	return nil
}

// Delete deletes a specific topic.
func (t *topicClient) Delete(topicName string) error {
	url := t.url + topicBaseURL
	err := t.client.DoHTTPJSONPBRequest(http.MethodDelete, url, nil, nil, withTopicName(topicName))
	if err != nil {
		return err
	}
	t.logger.Info("successfully deleted topic")
	return nil
}

// Get will get given topic.
func (t *topicClient) Get(topicName string) (m3topic.Topic, error) {
	var (
		url  = t.url + topicBaseURL
		resp = &admin.TopicGetResponse{}
	)
	err := t.client.DoHTTPJSONPBRequest(http.MethodGet, url, nil, resp, withTopicName(topicName))
	if err != nil {
		return nil, err
	}

	if resp.Topic == nil {
		return nil, errors.New("nil topic fetch")
	}
	t.logger.Debug("topic retrieved")
	return m3topic.NewTopicFromProto(resp.Topic)
}

// Add adds a consumer service to a given topic.
func (t *topicClient) Add(topicName string, consumerSvc *topicpb.ConsumerService) error {
	url := t.url + topicBaseURL
	req := &admin.TopicAddRequest{
		ConsumerService: consumerSvc,
	}

	err := t.client.DoHTTPJSONPBRequest(http.MethodPost, url, req, nil, withTopicName(topicName))
	if err != nil {
		return err
	}
	t.logger.Info("successfully added consumer to topic")
	return nil
}

func withTopicName(name string) m3admin.RequestOption {
	return m3admin.WithHeader(headerTopicName, name)
}
