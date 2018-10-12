// Copyright (c) 2016 Uber Technologies, Inc.
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

package eventer

import (
	"k8s.io/client-go/kubernetes"

	"go.uber.org/zap"
)

// Option provides configuration of an eventer.
type Option interface {
	execute(*options)
}

type options struct {
	kubeClient kubernetes.Interface
	logger     *zap.Logger
	namespace  string
	component  string
}

type optionFn func(o *options)

func (fn optionFn) execute(o *options) {
	fn(o)
}

// WithClient sets a client for the eventer (required).
func WithClient(c kubernetes.Interface) Option {
	return optionFn(func(o *options) {
		o.kubeClient = c
	})
}

// WithLogger sets a logger for the eventer. If not set a noop logger will
// be used.
func WithLogger(l *zap.Logger) Option {
	return optionFn(func(o *options) {
		o.logger = l
	})
}

// WithNamespace sets a namespace for the eventer.
func WithNamespace(ns string) Option {
	return optionFn(func(o *options) {
		o.namespace = ns
	})
}

// WithComponent sets a component for the eventer.
func WithComponent(c string) Option {
	return optionFn(func(o *options) {
		o.component = c
	})
}
