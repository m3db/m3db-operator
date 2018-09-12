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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/m3db/m3cluster/generated/proto/placementpb"
	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/gogo/protobuf/jsonpb"
	retryhttp "github.com/hashicorp/go-retryablehttp"
	plc "github.com/m3db/m3/src/query/api/v1/handler/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	"github.com/m3db/m3/src/query/util/logging"
	"go.uber.org/zap"
)

var ErrPlacementNotFound = errors.New("placement not found")

const (
	_defaultPlacementURI = "/api/v1/placement"
)

type placement struct {
	servicePort   int
	serviceName   string
	serviceDomain string
	client        *retryhttp.Client
	logger        *zap.Logger
}

type Option interface {
	execute(*placement) error
}

type optionFn func(p *placement) error

func (fn optionFn) execute(p *placement) error {
	return fn(p)
}

// WithPort is a setter to override the default port of 7201
func WithPort(port int) Option {
	return optionFn(func(p *placement) error {
		p.servicePort = port
		return nil
	})
}

// WithName is a setter to override the default name of m3coordinator
func WithName(name string) Option {
	return optionFn(func(p *placement) error {
		p.serviceName = name
		return nil
	})
}

// WithDomain is a setter to override the default name of default
func WithDomain(name string) Option {
	return optionFn(func(p *placement) error {
		p.serviceDomain = name
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

func New(opts ...Option) (Placement, error) {
	logging.InitWithCores(nil)
	ctx := context.Background()
	logger := logging.WithContext(ctx)
	// TODO(PS) Add logger.Sync() somewhere
	pl := &placement{
		client:        retryhttp.NewClient(),
		logger:        logger,
		servicePort:   m3admin.DefaultServicePort,
		serviceName:   m3admin.DefaultServiceName,
		serviceDomain: m3admin.DefaultServiceDomain,
	}
	for _, o := range opts {
		if err := o.execute(pl); err != nil {
			return nil, err
		}
	}
	return pl, nil
}

// formatDomain provides a formatted URL for HTTP reqeusts
func (p *placement) formatDomain() string {
	return fmt.Sprintf("http://%s.%s:%d", p.serviceName, p.serviceDomain, p.servicePort)
}

// Init will create the placement
func (p *placement) Init(req *admin.PlacementInitRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s", p.formatDomain(), plc.InitURL)
	_, err = m3admin.DoHttpRequest(p.client, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	p.logger.Info("successfully applied placement")
	return nil
}

// Delete will delete all current placements
func (p *placement) Delete() error {
	url := p.formatDomain()
	_, err := m3admin.DoHttpRequest(p.client, "DELETE", url, nil)
	if err != nil {
		return err
	}
	p.logger.Info("successfully deleted placement")
	return nil
}

// Get will get current placement
func (p *placement) Get() (*admin.PlacementGetResponse, error) {
	url := fmt.Sprintf("%s/%s", p.formatDomain(), plc.GetURL)
	resp, err := m3admin.DoHttpRequest(p.client, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPlacementNotFound
	}
	data := &admin.PlacementGetResponse{}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if err := jsonpb.Unmarshal(resp.Body, data); err != nil {
		return nil, err
	}
	p.logger.Info("placement retreived")
	return data, nil
}

// Add will add an instance to the current placement
func (p *placement) Add(instance placementpb.Instance) (*admin.PlacementGetResponse, error) {
	url := fmt.Sprintf("%s/%s", p.formatDomain(), plc.AddURL)
	data, err := json.Marshal(instance)
	if err != nil {
		return nil, err
	}
	resp, err := m3admin.DoHttpRequest(p.client, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	dataOut := &admin.PlacementGetResponse{}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if err := jsonpb.Unmarshal(resp.Body, dataOut); err != nil {
		return nil, err
	}
	p.logger.Info("successfully applied placement")
	return dataOut, nil
}
