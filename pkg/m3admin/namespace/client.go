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
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3/src/query/generated/proto/admin"

	"go.uber.org/zap"
)

const (
	namespaceBaseURL   = "/api/v1/namespace"
	namespaceDeleteFmt = namespaceBaseURL + "/%s"
)

type namespaceClient struct {
	client m3admin.Client
	logger *zap.Logger
	url    string
}

// Option provides an interface that can be used for setter options with the
// constructor
type Option interface {
	execute(*namespaceClient) error
}

type optionFn func(n *namespaceClient) error

func (fn optionFn) execute(n *namespaceClient) error {
	return fn(n)
}

// WithURL is a setter to override the default url of m3coordinator
func WithURL(u string) Option {
	return optionFn(func(n *namespaceClient) error {
		if _, err := url.ParseRequestURI(u); err != nil {
			return err
		}
		n.url = u
		return nil
	})
}

// WithLogger is a setter to override the default logger
func WithLogger(logger *zap.Logger) Option {
	return optionFn(func(n *namespaceClient) error {
		n.logger = logger
		return nil
	})
}

// WithClient configures an m3admin client.
func WithClient(cl m3admin.Client) Option {
	return optionFn(func(n *namespaceClient) error {
		n.client = cl
		return nil
	})
}

// NewClient constructs a new namespace client
func NewClient(opts ...Option) (Client, error) {
	logger := zap.NewNop()
	ns := &namespaceClient{
		client: m3admin.NewClient(),
		logger: logger,
	}
	for _, o := range opts {
		if err := o.execute(ns); err != nil {
			return nil, err
		}
	}
	return ns, nil
}

// Create will create a namespace
func (n *namespaceClient) Create(req *admin.NamespaceAddRequest) error {
	url := n.url + namespaceBaseURL
	err := n.client.DoHTTPJSONPBRequest(http.MethodPost, url, req, nil)
	if err != nil {
		return err
	}
	n.logger.Info("successfully created namespace", zap.String("namespace", req.Name))
	return nil
}

// List will retrieve all namespaces
func (n *namespaceClient) List() (*admin.NamespaceGetResponse, error) {
	var (
		url  = n.url + namespaceBaseURL
		resp = &admin.NamespaceGetResponse{}
	)
	err := n.client.DoHTTPJSONPBRequest(http.MethodGet, url, nil, resp)
	if err != nil {
		return nil, err
	}

	if resp.Registry == nil {
		n.logger.Error("nil registry from coordinator")
		return nil, errors.New("nil registry from coordinator")
	}

	n.logger.Debug("namespace retrieved")
	return data, nil
}

// Delete will delete a namespace
func (n *namespaceClient) Delete(namespace string) error {
	url := fmt.Sprintf(n.url+namespaceDeleteFmt, namespace)
	err := n.client.DoHTTPJSONPBRequest(http.MethodDelete, url, nil, nil)
	if err != nil {
		return err
	}
	n.logger.Info("successfully deleted namespace")
	return nil
}
