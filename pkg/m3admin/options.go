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
	logger *zap.Logger
	client *retryhttp.Client
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
