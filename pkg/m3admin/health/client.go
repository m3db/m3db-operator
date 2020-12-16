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
	"fmt"
	"net/http"
	"net/url"

	"github.com/m3db/m3db-operator/pkg/m3admin"
)

const (
	bootstrappedPath = "/bootstrapped"
	m3dbServiceName  = "m3dbnode-m3-cluster-short"
)

type healthClient struct {
	client m3admin.Client
	url    string
}

// Option provides an interface that can be used for setter options with the
// constructor
type Option interface {
	execute(*healthClient) error
}

type optionFn func(h *healthClient) error

func (fn optionFn) execute(h *healthClient) error {
	return fn(h)
}

// WithURL is a setter to override the default base url for a node.
func WithURL(u string) Option {
	return optionFn(func(h *healthClient) error {
		if u != "" {
			if _, err := url.ParseRequestURI(u); err != nil {
				return err
			}
		}
		h.url = u
		return nil
	})
}

// WithClient configures an m3admin client.
func WithClient(cl m3admin.Client) Option {
	return optionFn(func(h *healthClient) error {
		h.client = cl
		return nil
	})
}

// NewClient constructs a new health client.
func NewClient(opts ...Option) (Client, error) {
	hc := &healthClient{
		client: m3admin.NewClient(),
	}
	for _, o := range opts {
		if err := o.execute(hc); err != nil {
			return nil, err
		}
	}
	return hc, nil
}

func (h *healthClient) Bootstrapped(namespace string, podName string, port int) (bool, error) {
	url := h.url
	if url == "" {
		url = getPodURL(namespace, podName, port)
	}

	_, err := h.client.DoHTTPRequest(http.MethodGet, url+bootstrappedPath, nil)
	if err != nil {
		return false, err
	}

	return true, nil
}

func getPodURL(namespace string, podName string, port int) string {
	return fmt.Sprintf("http://%s.%s.%s:%d", podName, m3dbServiceName, namespace, port)
}
