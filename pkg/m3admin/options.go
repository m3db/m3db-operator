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
	retryhttp "github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

// Option configures an m3admin client.
type Option interface {
	execute(*options)
}

type optionFn func(o *options)

func (f optionFn) execute(o *options) {
	f(o)
}

type options struct {
	logger      *zap.Logger
	client      *retryhttp.Client
	environment string
	zone        string
}

// WithLogger configures a logger for the client. If not set a noop logger will
// be used.
func WithLogger(l *zap.Logger) Option {
	return optionFn(func(o *options) {
		o.logger = l
	})
}

// WithHTTPClient configures an http client for the m3admin client. If not set,
// go-retryablehttp's default client will be used. Note that the retry client's
// logger will be overridden with the stdlog-wrapped wrapped zap logger.
func WithHTTPClient(cl *retryhttp.Client) Option {
	return optionFn(func(o *options) {
		o.client = cl
	})
}

// WithEnvironment controls the Cluster-Environment-Name header to
// m3coordinator.
func WithEnvironment(e string) Option {
	return optionFn(func(o *options) {
		o.environment = e
	})
}

// WithZone controls the Cluster-Zone-Name header to m3coordinator.
func WithZone(z string) Option {
	return optionFn(func(o *options) {
		o.zone = z
	})
}

// RequestOption defines a per-request option.
type RequestOption interface {
	execute(*reqOptions)
}

type reqOptionFn func(o *reqOptions)

func (f reqOptionFn) execute(o *reqOptions) {
	f(o)
}

type reqOptions struct {
	headers map[string]string
}

// WithHeader adds a header to a request. It can be specified multiple times.
func WithHeader(name, value string) RequestOption {
	return reqOptionFn(func(o *reqOptions) {
		if o.headers == nil {
			o.headers = make(map[string]string)
		}
		o.headers[name] = value
	})
}
