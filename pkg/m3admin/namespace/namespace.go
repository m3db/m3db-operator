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
	"errors"
	"fmt"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

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
		return &admin.NamespaceAddRequest{
			Name:    ns.Name,
			Options: requestOptsFromAPI(ns.Options),
		}, nil
	}

	var opts myspec.NamespaceOptions
	switch ns.Preset {
	case string(PresetTenSecondsTwoDaysIndexed):
		opts = presetTenSecondsTwoDaysIndexed
	default:
		return nil, fmt.Errorf("preset '%s' not found", ns.Preset)
	}

	return &admin.NamespaceAddRequest{
		Name:    ns.Name,
		Options: requestOptsFromAPI(&opts),
	}, nil
}

func requestOptsFromAPI(opts *myspec.NamespaceOptions) *m3ns.NamespaceOptions {
	return &m3ns.NamespaceOptions{
		BootstrapEnabled:  opts.BootstrapEnabled,
		FlushEnabled:      opts.FlushEnabled,
		WritesToCommitLog: opts.WritesToCommitLog,
		CleanupEnabled:    opts.CleanupEnabled,
		RepairEnabled:     opts.RepairEnabled,
		RetentionOptions:  retentionOptsFromAPI(opts.RetentionOptions),
		SnapshotEnabled:   opts.SnapshotEnabled,
		IndexOptions:      indexOptsFromAPI(opts.IndexOptions),
	}
}

func retentionOptsFromAPI(opts myspec.RetentionOptions) *m3ns.RetentionOptions {
	return &m3ns.RetentionOptions{
		RetentionPeriodNanos:                     opts.RetentionPeriod.Nanoseconds(),
		BlockSizeNanos:                           opts.BlockSize.Nanoseconds(),
		BufferFutureNanos:                        opts.BufferFuture.Nanoseconds(),
		BufferPastNanos:                          opts.BufferPast.Nanoseconds(),
		BlockDataExpiry:                          opts.BlockDataExpiry,
		BlockDataExpiryAfterNotAccessPeriodNanos: opts.BlockDataExpiryAfterNotAccessPeriod.Nanoseconds(),
	}
}

func indexOptsFromAPI(opts myspec.IndexOptions) *m3ns.IndexOptions {
	return &m3ns.IndexOptions{
		Enabled:        opts.Enabled,
		BlockSizeNanos: opts.BlockSize.Nanoseconds(),
	}
}
