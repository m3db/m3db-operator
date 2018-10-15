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
	"net/url"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"go.uber.org/zap"
)

// Option provides an interface that can be used for setter options with the
// constructor
type Option interface {
	execute(*placementClient) error
}

type optionFn func(p *placementClient) error

func (fn optionFn) execute(p *placementClient) error {
	return fn(p)
}

// WithURL is a setter to override the default URL
func WithURL(u string) Option {
	return optionFn(func(p *placementClient) error {
		if _, err := url.ParseRequestURI(u); err != nil {
			return err
		}
		p.url = u
		return nil
	})
}

// WithLogger is a setter to override the default logger
func WithLogger(logger *zap.Logger) Option {
	return optionFn(func(p *placementClient) error {
		p.logger = logger
		return nil
	})
}

// WithClient configures an m3admin client.
func WithClient(cl m3admin.Client) Option {
	return optionFn(func(p *placementClient) error {
		p.client = cl
		return nil
	})
}
