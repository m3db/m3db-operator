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
	"errors"
	"fmt"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"

	"github.com/gogo/protobuf/jsonpb"
	pbtypes "github.com/gogo/protobuf/types"
	m3ns "github.com/m3db/m3/src/dbnode/generated/proto/namespace"
	"github.com/m3db/m3/src/query/generated/proto/admin"
)

// RequestFromSpec returns a namespace add request from a cluster spec namespace
// config.
func RequestFromSpec(ns myspec.Namespace) (*admin.NamespaceAddRequest, error) {
	if ns.Name == "" {
		return nil, errors.New("must set namespace name")
	}

	if ns.Preset != "" && ns.Options != nil {
		return nil, errors.New("must set only one of preset or options")
	}

	if ns.Preset == "" && ns.Options == nil {
		return nil, errors.New("must set either preset or options")
	}

	if ns.Options != nil {
		opts, err := m3dbNamespaceOptsFromSpec(ns.Options)
		if err != nil {
			return nil, err
		}

		return &admin.NamespaceAddRequest{
			Name:    ns.Name,
			Options: opts,
		}, nil
	}

	var opts myspec.NamespaceOptions
	switch ns.Preset {
	case string(PresetTenSecondsTwoDaysIndexed):
		opts = presetTenSecondsTwoDaysIndexed
	case string(PresetOneMinuteFourtyDaysIndexed):
		opts = presetOneMinuteFourtyDaysIndexed
	default:
		return nil, fmt.Errorf("preset '%s' not found", ns.Preset)
	}

	requestOpts, err := m3dbNamespaceOptsFromSpec(&opts)
	if err != nil {
		return nil, err
	}

	return &admin.NamespaceAddRequest{
		Name:    ns.Name,
		Options: requestOpts,
	}, nil
}

func m3dbNamespaceOptsFromSpec(opts *myspec.NamespaceOptions) (*m3ns.NamespaceOptions, error) {
	retentionOpts, err := m3dbRetentionOptsFromSpec(opts.RetentionOptions)
	if err != nil {
		return nil, err
	}

	indexOpts, err := m3dbIndexOptsFromSpec(opts.IndexOptions)
	if err != nil {
		return nil, err
	}

	aggOpts, err := m3dbAggregationOptsFromSpec(opts.AggregationOptions)
	if err != nil {
		return nil, err
	}

	extOpts, err := m3dbExtendedOptsFromSpec(opts.ExtendedOptions)
	if err != nil {
		return nil, err
	}

	return &m3ns.NamespaceOptions{
		BootstrapEnabled:   opts.BootstrapEnabled,
		FlushEnabled:       opts.FlushEnabled,
		WritesToCommitLog:  opts.WritesToCommitLog,
		CleanupEnabled:     opts.CleanupEnabled,
		RepairEnabled:      opts.RepairEnabled,
		RetentionOptions:   retentionOpts,
		SnapshotEnabled:    opts.SnapshotEnabled,
		IndexOptions:       indexOpts,
		ColdWritesEnabled:  opts.ColdWritesEnabled,
		AggregationOptions: aggOpts,
		ExtendedOptions:    extOpts,
	}, nil
}

func m3dbRetentionOptsFromSpec(opts myspec.RetentionOptions) (*m3ns.RetentionOptions, error) {
	retention, err := time.ParseDuration(opts.RetentionPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retention option RetentionPeriod: %v", err)
	}

	blockSize, err := time.ParseDuration(opts.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retention option BlockSize: %v", err)
	}

	bufferFuture, err := time.ParseDuration(opts.BufferFuture)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retention option BufferFuture: %v", err)
	}

	bufferPast, err := time.ParseDuration(opts.BufferPast)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retention option BufferPast: %v", err)
	}

	expiryNanos, err := time.ParseDuration(opts.BlockDataExpiryAfterNotAccessPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retention option BlockDataExpiryAfterNotAccessPeriod: %v", err)
	}

	return &m3ns.RetentionOptions{
		RetentionPeriodNanos:                     retention.Nanoseconds(),
		BlockSizeNanos:                           blockSize.Nanoseconds(),
		BufferFutureNanos:                        bufferFuture.Nanoseconds(),
		BufferPastNanos:                          bufferPast.Nanoseconds(),
		BlockDataExpiry:                          opts.BlockDataExpiry,
		BlockDataExpiryAfterNotAccessPeriodNanos: expiryNanos.Nanoseconds(),
	}, nil
}

func m3dbIndexOptsFromSpec(opts myspec.IndexOptions) (*m3ns.IndexOptions, error) {
	blockSize, err := time.ParseDuration(opts.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse index option BlockSize: %v", err)
	}

	return &m3ns.IndexOptions{
		Enabled:        opts.Enabled,
		BlockSizeNanos: blockSize.Nanoseconds(),
	}, nil
}

func m3dbAggregationOptsFromSpec(opts myspec.AggregationOptions) (*m3ns.AggregationOptions, error) {
	if len(opts.Aggregations) == 0 {
		return nil, nil
	}

	aggs := make([]*m3ns.Aggregation, 0, len(opts.Aggregations))
	for _, specAgg := range opts.Aggregations {
		agg := &m3ns.Aggregation{Aggregated: specAgg.Aggregated}
		if agg.Aggregated {
			resolution, err := time.ParseDuration(specAgg.Attributes.Resolution)
			if err != nil {
				return nil, fmt.Errorf("failed to parse aggregation option Resolution: %w", err)
			}

			agg.Attributes = &m3ns.AggregatedAttributes{
				ResolutionNanos: resolution.Nanoseconds(),
			}

			if specAgg.Attributes.DownsampleOptions == nil {
				agg.Attributes.DownsampleOptions = &m3ns.DownsampleOptions{All: true}
			} else {
				agg.Attributes.DownsampleOptions = &m3ns.DownsampleOptions{
					All: specAgg.Attributes.DownsampleOptions.All,
				}
			}
		}

		aggs = append(aggs, agg)
	}

	return &m3ns.AggregationOptions{
		Aggregations: aggs,
	}, nil

}

func m3dbExtendedOptsFromSpec(opts *myspec.ExtendedOptions) (*m3ns.ExtendedOptions, error) {
	if opts == nil {
		return nil, nil
	}

	pbStruct, err := pbStructFromSpec(opts.Options)
	if err != nil {
		return nil, err
	}

	return &m3ns.ExtendedOptions{
		Type:    opts.Type,
		Options: pbStruct,
	}, nil
}

func pbStructFromSpec(opts map[string]interface{}) (*pbtypes.Struct, error) {
	jsonBytes, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	pbStruct := &pbtypes.Struct{}
	if err := jsonpb.Unmarshal(bytes.NewReader(jsonBytes), pbStruct); err != nil {
		return nil, err
	}

	return pbStruct, nil
}
