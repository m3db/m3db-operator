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

package namespace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"github.com/m3db/m3db-operator/pkg/m3admin"

	namespacepb "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	ns "github.com/m3db/m3/src/dbnode/storage/namespace"
	nsh "github.com/m3db/m3/src/query/api/v1/handler/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"

	"github.com/gogo/protobuf/jsonpb"
	"go.uber.org/zap"
)

const (
	// TODO(PS) add to spec for crd?
	// defaults for namespace creation
	_defaultBootstrapEnabled                                         = true
	_defaultFlushEnabled                                             = true
	_defaultWritesToCommitLog                                        = true
	_defaultCleanupEnabled                                           = true
	_defaultSnapshotEnabled                                          = false
	_defaultRepairEnabled                                            = false
	_defaultRententionOptionRetentionPeriodNanos                     = int64(172800000000000)
	_defaultRententionOptionBlockSizeNanos                           = int64(7200000000000)
	_defaultRententionOptionBufferFutureNanos                        = int64(600000000000)
	_defaultRententionOptionBufferPastNanos                          = int64(600000000000)
	_defaultRententionOptionBlockDataExpiry                          = true
	_defaultRententionOptionBlockDataExpiryAfterNotAccessPeriodNanos = int64(300000000000)
	_defaultIndexOptionenabled                                       = true
	_defaultIndexOptionBlockSizeNanos                                = int64(7200000000000)
)

type namespaceClient struct {
	client m3admin.Client
	logger *zap.Logger
	url    string
}

// Option provides an interface that can be used for setter options with the
// constructor
type Option interface {
	execute(*namespaceClient) error
}

type optionFn func(n *namespaceClient) error

func (fn optionFn) execute(n *namespaceClient) error {
	return fn(n)
}

// WithURL is a setter to override the default url of m3coordinator
func WithURL(u string) Option {
	return optionFn(func(n *namespaceClient) error {
		if _, err := url.ParseRequestURI(u); err != nil {
			return err
		}
		n.url = u
		return nil
	})
}

// WithLogger is a setter to override the default logger
func WithLogger(logger *zap.Logger) Option {
	return optionFn(func(n *namespaceClient) error {
		n.logger = logger
		return nil
	})
}

// WithClient configures an m3admin client.
func WithClient(cl m3admin.Client) Option {
	return optionFn(func(n *namespaceClient) error {
		n.client = cl
		return nil
	})
}

// NewClient constructs a new namespace client
func NewClient(opts ...Option) (Client, error) {
	logger := zap.NewNop()
	ns := &namespaceClient{
		client: m3admin.NewClient(),
		logger: logger,
		url:    m3admin.DefaultURL,
	}
	for _, o := range opts {
		if err := o.execute(ns); err != nil {
			return nil, err
		}
	}
	return ns, nil
}

// defaultNamespaceReqeust provides sane defaults for a namespace reqeust
func defaultNamespaceRequest(namespaceName string) *admin.NamespaceAddRequest {
	return &admin.NamespaceAddRequest{
		Name: namespaceName,
		Options: &namespacepb.NamespaceOptions{
			BootstrapEnabled:  _defaultBootstrapEnabled,
			FlushEnabled:      _defaultFlushEnabled,
			WritesToCommitLog: _defaultWritesToCommitLog,
			CleanupEnabled:    _defaultCleanupEnabled,
			RepairEnabled:     _defaultRepairEnabled,
			RetentionOptions: &namespacepb.RetentionOptions{
				RetentionPeriodNanos:                     _defaultRententionOptionRetentionPeriodNanos,
				BlockSizeNanos:                           _defaultRententionOptionBlockSizeNanos,
				BufferFutureNanos:                        _defaultRententionOptionBufferFutureNanos,
				BufferPastNanos:                          _defaultRententionOptionBufferPastNanos,
				BlockDataExpiry:                          _defaultRententionOptionBlockDataExpiry,
				BlockDataExpiryAfterNotAccessPeriodNanos: _defaultRententionOptionBlockDataExpiryAfterNotAccessPeriodNanos,
			},
			SnapshotEnabled: _defaultSnapshotEnabled,
			IndexOptions: &namespacepb.IndexOptions{
				Enabled:        _defaultIndexOptionenabled,
				BlockSizeNanos: _defaultIndexOptionBlockSizeNanos,
			},
		},
	}
}

// Create will create a namespace
func (n *namespaceClient) Create(namespaceName string) error {
	url := fmt.Sprintf("%s%s", n.url, nsh.AddURL)
	data, err := json.Marshal(defaultNamespaceRequest(namespaceName))
	if err != nil {
		return err
	}
	_, err = n.client.DoHTTPRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	n.logger.Info("successfully created namespace", zap.String("namespace", namespaceName))
	return nil
}

// List will retrieve all namespaces
func (n *namespaceClient) List() ([]ns.Metadata, error) {
	url := fmt.Sprintf("%s%s", n.url, nsh.GetURL)
	resp, err := n.client.DoHTTPRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	data := &admin.NamespaceGetResponse{}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if err := jsonpb.Unmarshal(resp.Body, data); err != nil {
		return nil, err
	}
	nsMetas := []ns.Metadata{}
	for nsID, nsOpts := range data.GetRegistry().GetNamespaces() {
		md, err := ns.ToMetadata(nsID, nsOpts)
		if err != nil {
			return nil, err
		}
		nsMetas = append(nsMetas, md)
	}
	n.logger.Info("namespace retrieved")
	return nsMetas, nil
}

// Delete will delete a namespace
func (n *namespaceClient) Delete(namespace string) error {
	url := fmt.Sprintf("%s%s/%s", n.url, nsh.AddURL, namespace)
	_, err := n.client.DoHTTPRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	n.logger.Info("successfully deleted namespace")
	return nil
}
