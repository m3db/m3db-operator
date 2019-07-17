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
	"errors"
	"fmt"
	"net/http"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"
	m3placement "github.com/m3db/m3/src/cluster/placement"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	"go.uber.org/zap"
)

const (
	placementBaseURL    = "/api/v1/services/m3db/placement"
	placementInitURL    = placementBaseURL + "/init"
	placementReplaceURL = placementBaseURL + "/replace"
	placementRemoveFmt  = placementBaseURL + "/%s"
)

type placementClient struct {
	url    string
	client m3admin.Client
	logger *zap.Logger
}

// NewClient is the constructor the Placement interface
func NewClient(opts ...Option) (Client, error) {
	logger := zap.NewNop()
	pl := &placementClient{
		client: m3admin.NewClient(),
		logger: logger,
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
	url := p.url + placementInitURL
	err := p.client.DoHTTPJSONPBRequest(http.MethodPost, url, req, nil)
	if err != nil {
		return err
	}
	p.logger.Info("successfully applied placement")
	return nil
}

// Delete will delete all current placements
func (p *placementClient) Delete() error {
	url := p.url + placementBaseURL
	err := p.client.DoHTTPJSONPBRequest(http.MethodDelete, url, nil, nil)
	if err != nil {
		return err
	}
	p.logger.Info("successfully deleted placement")
	return nil
}

// Get will get current placement
func (p *placementClient) Get() (m3placement.Placement, error) {
	var (
		url  = p.url + placementBaseURL
		resp = &admin.PlacementGetResponse{}
	)
	err := p.client.DoHTTPJSONPBRequest(http.MethodGet, url, nil, resp)
	if err != nil {
		return nil, err
	}

	if resp.Placement == nil {
		return nil, errors.New("nil placement fetch")
	}
	p.logger.Debug("placement retrieved")
	return m3placement.NewPlacementFromProto(resp.Placement)
}

// Add will add an instance to the current placement
func (p *placementClient) Add(instance placementpb.Instance) error {
	url := p.url + placementBaseURL
	req := &admin.PlacementAddRequest{
		Instances: []*placementpb.Instance{&instance},
	}

	err := p.client.DoHTTPJSONPBRequest(http.MethodPost, url, req, nil)
	if err != nil {
		return err
	}
	p.logger.Debug("successfully add instance to placement")
	return nil
}

func (p *placementClient) Remove(id string) error {
	url := fmt.Sprintf(p.url+placementRemoveFmt, id)
	return p.client.DoHTTPJSONPBRequest(http.MethodDelete, url, nil, nil)
}

func (p *placementClient) Replace(leavingInstanceID string, newInst placementpb.Instance) error {
	url := p.url + placementReplaceURL
	req := &admin.PlacementReplaceRequest{
		LeavingInstanceIDs: []string{leavingInstanceID},
		Candidates:         []*placementpb.Instance{&newInst},
	}
	return p.client.DoHTTPJSONPBRequest(http.MethodPost, url, req, nil)
}
