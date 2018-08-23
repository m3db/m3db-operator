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

package m3admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	namespace "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"
	placementpb "github.com/m3db/m3cluster/generated/proto/placementpb"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
)

const (
	_httpTimeout = 30 * time.Second

	_m3coordinatorServiceName = "m3coordinator"
	_m3dbnodeServiceName      = "m3dbnode"

	// defaults for placement init request
	_placementInitURI = "/api/v1/placement/init"
	_placementURI     = "/api/v1/placement"
	_defaultZoneType  = "embedded"
	_defaultWeight    = 100
	_defaultM3DBPort  = 9000

	// TODO(PS) add to spec for crd?
	// defaults for namespace creation
	_namespaceURI                                                    = "/api/v1/namespace"
	_defaultName                                                     = "default"
	_defaultBootstrapEnabled                                         = true
	_defaultFlushEnabled                                             = true
	_defaultWritesToCommitLog                                        = true
	_defaultCleanupEnabled                                           = true
	_defaultSnapshotEnabled                                          = false
	_defaultRepairEnabled                                            = false
	_defaultRententionOptionRetentionPeriodNanos                     = 172800000000000
	_defaultRententionOptionBlockSizeNanos                           = 7200000000000
	_defaultRententionOptionBufferFutureNanos                        = 600000000000
	_defaultRententionOptionBufferPastNanos                          = 600000000000
	_defaultRententionOptionBlockDataExpiry                          = true
	_defaultRententionOptionBlockDataExpiryAfterNotAccessPeriodNanos = 300000000000
	_defaultIndexOptionenabled                                       = true
	_defaultIndexOptionBlockSizeNanos                                = 7200000000000
)

// M3admin is the interface to the perform M3 admin operations
type M3admin interface {
	PlacementInit(cluster *myspec.M3DBCluster, instances map[string]string) error
	PlacementDelete(cluster *myspec.M3DBCluster) error
	NamespaceCreate(cluster *myspec.M3DBCluster) error
	NamespaceDelete(cluster *myspec.M3DBCluster, namespace string) error
}

type m3admin struct {
	httpClient *http.Client
}

// New constructs a new M3Admin interface
func New() M3admin {
	return &m3admin{
		httpClient: &http.Client{Timeout: _httpTimeout},
	}
}

//NamespaceCreate will create a namespace
func (m *m3admin) NamespaceCreate(cluster *myspec.M3DBCluster) error {
	namespaceCreateRequest := admin.NamespaceAddRequest{
		Name: _defaultName,
		Options: &namespace.NamespaceOptions{
			BootstrapEnabled:  _defaultBootstrapEnabled,
			FlushEnabled:      _defaultFlushEnabled,
			WritesToCommitLog: _defaultWritesToCommitLog,
			CleanupEnabled:    _defaultCleanupEnabled,
			RepairEnabled:     _defaultRepairEnabled,
			RetentionOptions: &namespace.RetentionOptions{
				RetentionPeriodNanos:                     _defaultRententionOptionRetentionPeriodNanos,
				BlockSizeNanos:                           _defaultRententionOptionBlockSizeNanos,
				BufferFutureNanos:                        _defaultRententionOptionBufferFutureNanos,
				BufferPastNanos:                          _defaultRententionOptionBufferPastNanos,
				BlockDataExpiry:                          _defaultRententionOptionBlockDataExpiry,
				BlockDataExpiryAfterNotAccessPeriodNanos: _defaultRententionOptionBlockDataExpiryAfterNotAccessPeriodNanos,
			},
			SnapshotEnabled: _defaultSnapshotEnabled,
			IndexOptions: &namespace.IndexOptions{
				Enabled:        _defaultIndexOptionenabled,
				BlockSizeNanos: _defaultIndexOptionBlockSizeNanos,
			},
		},
	}
	data, err := json.Marshal(namespaceCreateRequest)
	if err != nil {
		return err
	}
	// TODO(PS) - Remove hardcoded references
	url := fmt.Sprintf("http://%s.%s:7201%s", _m3coordinatorServiceName, cluster.GetNamespace(), _namespaceURI)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	if err := m.doHttpRequest(request); err != nil {
		return err
	}
	logrus.WithField("namespace", namespaceCreateRequest.GetName()).Info("successfully created namespace")

	return nil
}

// NamespaceDelete will delete a namespace
func (m *m3admin) NamespaceDelete(cluster *myspec.M3DBCluster, namespace string) error {
	// TODO(PS) - Remove hardcoded references
	url := fmt.Sprintf("http://%s.%s:7201%s/%s", _m3coordinatorServiceName, cluster.GetNamespace(), _namespaceURI, namespace)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	if err := m.doHttpRequest(request); err != nil {
		return err
	}
	logrus.Info("successfully deleted namespace")
	return nil
}

// PlacementInit will create the placement
func (m *m3admin) PlacementInit(cluster *myspec.M3DBCluster, instances map[string]string) error {
	placementInitRequest := &admin.PlacementInitRequest{
		NumShards:         cluster.Spec.NumberOfShards,
		ReplicationFactor: cluster.Spec.ReplicationFactor,
	}
	for hostname, zone := range instances {
		fqdnHostname := fmt.Sprintf("%s.%s", hostname, _m3dbnodeServiceName)
		instance := &placementpb.Instance{
			Id:             hostname,
			IsolationGroup: zone,
			Zone:           _defaultZoneType,
			Weight:         _defaultWeight,
			Hostname:       fqdnHostname,
			Endpoint:       fmt.Sprintf("%s:%d", fqdnHostname, _defaultM3DBPort),
			Port:           _defaultM3DBPort,
		}
		placementInitRequest.Instances = append(placementInitRequest.Instances, instance)
	}
	data, err := json.Marshal(placementInitRequest)
	if err != nil {
		return err
	}
	// TODO(PS) - Remove hardcoded references
	url := fmt.Sprintf("http://%s.%s:7201%s", _m3coordinatorServiceName, cluster.GetNamespace(), _placementInitURI)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	if err := m.doHttpRequest(request); err != nil {
		return err
	}
	logrus.Info("successfully applied placement")
	return nil
}

// PlacementDelete will delete current placement
func (m *m3admin) PlacementDelete(cluster *myspec.M3DBCluster) error {
	// TODO(PS) - Remove hardcoded references
	url := fmt.Sprintf("http://%s.%s:7201%s", _m3coordinatorServiceName, cluster.GetNamespace(), _placementURI)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	if err := m.doHttpRequest(request); err != nil {
		return err
	}
	logrus.Info("successfully deleted placement")
	return nil
}

// doHttpReqeust is a simple helper for HTTP requests
func (m *m3admin) doHttpRequest(request *http.Request) error {
	response, err := m.httpClient.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return errors.New("http response was not status ok")
	}
	return nil
}
