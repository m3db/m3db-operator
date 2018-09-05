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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	plc "github.com/m3db/m3/src/query/api/v1/handler/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"
	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/gogo/protobuf/jsonpb"
	retryhttp "github.com/hashicorp/go-retryablehttp"
	"go.uber.org/zap"
)

const (
	_defaultPlacementURI = "/api/v1/placement"
)

type placement struct {
	url    string
	client *retryhttp.Client
	logger *zap.Logger
}

// Option provides an interface that can be used for setter options with the
// constructor
type Option interface {
	execute(*placement) error
}

type optionFn func(p *placement) error

func (fn optionFn) execute(p *placement) error {
	return fn(p)
}

// WithURL is a setter to override the default URL
func WithURL(u string) Option {
	return optionFn(func(p *placement) error {
		if _, err := url.ParseRequestURI(u); err != nil {
			return err
		}
		p.url = u
		return nil
	})
}

// WithLogger is a setter to override the default logger
func WithLogger(logger *zap.Logger) Option {
	return optionFn(func(p *placement) error {
		p.logger = logger
		return nil
	})
}

// NewClient is the constructor the Placement interface
func NewClient(opts ...Option) (Client, error) {
	logger := zap.NewNop()
	pl := &placement{
		client: retryhttp.NewClient(),
		logger: logger,
		url:    m3admin.DefaultURL,
	}
	for _, o := range opts {
		if err := o.execute(pl); err != nil {
			return nil, err
		}
	}
	return pl, nil
}

// Init will create the placement
func (p *placement) Init(req *admin.PlacementInitRequest) error {
	url := fmt.Sprintf("%s%s", p.url, plc.InitURL)
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = m3admin.DoHTTPRequest(p.client, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	p.logger.Info("successfully applied placement")
	return nil
}

// Delete will delete all current placements
func (p *placement) Delete() error {
	url := fmt.Sprintf("%s%s", p.url, plc.GetURL)
	_, err := m3admin.DoHTTPRequest(p.client, "DELETE", url, nil)
	if err != nil {
		return err
	}
	p.logger.Info("successfully deleted placement")
	return nil
}

// Get will get current placement
func (p *placement) Get() (string, error) {
	url := fmt.Sprintf("%s%s", p.url, plc.GetURL)
	resp, err := m3admin.DoHTTPRequest(p.client, "GET", url, nil)
	if err != nil {
		return "", err
	}
	data := &admin.PlacementGetResponse{}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if err := jsonpb.Unmarshal(resp.Body, data); err != nil {
		return "", err
	}
	p.logger.Info("placement retreived")
	return data.GetPlacement().String(), nil
}

// Add will add an instance to the current placement
func (p *placement) Add(instance placementpb.Instance) error {
	url := fmt.Sprintf("%s%s", p.url, plc.AddURL)
	data, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	_, err = m3admin.DoHTTPRequest(p.client, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	p.logger.Info("successfully add instance to placement")
	return nil
}
