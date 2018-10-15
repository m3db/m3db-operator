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
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	plc "github.com/m3db/m3/src/query/api/v1/handler/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3cluster/generated/proto/placementpb"
	m3placement "github.com/m3db/m3cluster/placement"

	"github.com/gogo/protobuf/jsonpb"
	"go.uber.org/zap"
)

type placementClient struct {
	url    string
	client m3admin.Client
	logger *zap.Logger
}

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

// NewClient is the constructor the Placement interface
func NewClient(opts ...Option) (Client, error) {
	logger := zap.NewNop()
	pl := &placementClient{
		client: m3admin.NewClient(),
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
func (p *placementClient) Init(req *admin.PlacementInitRequest) error {
	url := fmt.Sprintf("%s%s", p.url, plc.M3DBInitURL)
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = p.client.DoHTTPRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	p.logger.Info("successfully applied placement")
	return nil
}

// Delete will delete all current placements
func (p *placementClient) Delete() error {
	url := fmt.Sprintf("%s%s", p.url, plc.M3DBGetURL)
	_, err := p.client.DoHTTPRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	p.logger.Info("successfully deleted placement")
	return nil
}

// Get will get current placement
func (p *placementClient) Get() (m3placement.Placement, error) {
	url := fmt.Sprintf("%s%s", p.url, plc.M3DBGetURL)
	resp, err := p.client.DoHTTPRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	data := &admin.PlacementGetResponse{}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if err := jsonpb.Unmarshal(resp.Body, data); err != nil {
		return nil, err
	}
	if data.Placement == nil {
		return nil, errors.New("nil placement fetch")
	}
	p.logger.Info("placement retreived")
	return m3placement.NewPlacementFromProto(data.Placement)
}

// Add will add an instance to the current placement
func (p *placementClient) Add(instance placementpb.Instance) error {
	url := fmt.Sprintf("%s%s", p.url, plc.M3DBAddURL)
	request := &admin.PlacementAddRequest{
		Instances: []*placementpb.Instance{&instance},
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = p.client.DoHTTPRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	p.logger.Info("successfully add instance to placement")
	return nil
}
