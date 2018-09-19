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
	"fmt"
	"net/http"

	retryhttp "github.com/hashicorp/go-retryablehttp"
)

const (
	// DefaultServicePort is the typical default of the M3Coordinator service
	DefaultServicePort = 7201

	// DefaultServiceName is the typical name of the M3Coordinator service
	DefaultServiceName = "m3coordinator"

	// DefaultServiceDomain is the typical domain of the M3Coordinator service
	DefaultServiceDomain = "default"
)

var (
	ErrNotOk    = errors.New("status not ok")
	ErrNotFound = errors.New("status not found")
)

// DoHTTPRequest is a simple helper for HTTP requests
func DoHTTPRequest(
	client *retryhttp.Client,
	action, url string,
	data *bytes.Buffer,
) (*http.Response, error) {
	var request *retryhttp.Request
	var err error
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
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	fmt.Println(fmt.Sprintf("status: %+v", response.Status))
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, ErrNotOk
	}
	return response, nil
}
