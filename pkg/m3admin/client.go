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
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httputil"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// DefaultURL is the typical URL for the namespace service
	DefaultURL = "http://m3coordinator.default:7201"
)

var (
	// ErrNotOk indicates that HTTP status was not Ok
	ErrNotOk = errors.New("status not ok")

	// ErrNotFound indicates that HTTP status was not found
	ErrNotFound = errors.New("status not found")
)

// Client is an m3admin client.
type Client interface {
	DoHTTPRequest(action, url string, data *bytes.Buffer) (*http.Response, error)
}

type client struct {
	client *retryhttp.Client
	logger *zap.Logger
}

// NewClient returns a new m3admin client.
func NewClient(clientOpts ...Option) Client {
	opts := &options{}
	for _, o := range clientOpts {
		o.execute(opts)
	}

	client := &client{
		client: opts.client,
		logger: opts.logger,
	}

	if client.client == nil {
		client.client = retryhttp.NewClient()
	}
	if client.logger == nil {
		client.logger = zap.NewNop()
	}

	// We do our own request logging, silence their logger.
	client.client.Logger.SetOutput(ioutil.Discard)
	client.client.ErrorHandler = retryhttp.PassthroughErrorHandler

	return client
}

// DoHTTPRequest is a simple helper for HTTP requests
func (c *client) DoHTTPRequest(
	action, url string,
	data *bytes.Buffer,
) (*http.Response, error) {

	l := c.logger.With(zap.String("action", action), zap.String("url", url))

	var request *retryhttp.Request
	var err error

	// retryhttp type switches on the data parameter, if data is types as
	// *bytes.Buffer but nil it will panic
	if data == nil {
		request, err = retryhttp.NewRequest(action, url, nil)
		if err != nil {
			return nil, err
		}
	} else {
		request, err = retryhttp.NewRequest(action, url, data)
		if err != nil {
			return nil, err
		}
	}

	request.Header.Add("Content-Type", "application/json")

	if l.Core().Enabled(zapcore.DebugLevel) {
		dump, err := httputil.DumpRequest(request.Request, true)
		if err != nil {
			l = l.With(zap.String("requestDumpError", err.Error()))
		} else {
			l = l.With(zap.ByteString("requestDump", dump))
		}
	}

	response, err := c.client.Do(request)
	if err != nil {
		l.Debug("request error", zap.Error(err))
		return nil, err
	}

	l = l.With(zap.String("status", response.Status))

	// If in debug mode, dump the entire request+response (coordinator error
	// messages are included in it).
	if l.Core().Enabled(zapcore.DebugLevel) {
		dump, err := httputil.DumpResponse(response, true)
		if err != nil {
			l = l.With(zap.String("responseDumpError", err.Error()))
		} else {
			l = l.With(zap.ByteString("responseDump", dump))
		}
	}

	l.Debug("response received")

	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, ErrNotOk
	}
	return response, nil
}
