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
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
)

// Preset represents a predfined namespace.
type Preset string

const (
	// PresetTenSecondsTwoDaysIndexed stores metrics at 10 second resolution for
	// two days with a block size of 2h. BufferPast and BufferFuture are both 10m.
	// Snapshotting is enabled. Metrics are indexed based on tags.
	PresetTenSecondsTwoDaysIndexed Preset = "10s:2d"

	// PresetOneMinuteFourtyDaysIndexed stores metrics at 10 second resolution for
	// two days with a block size of 2h. BufferPast and BufferFuture are both 10m.
	// Snapshotting is enabled. Metrics are indexed based on tags.
	PresetOneMinuteFourtyDaysIndexed Preset = "1m:40d"
)

var (
	presetTenSecondsTwoDaysIndexed = myspec.NamespaceOptions{
		BootstrapEnabled:  true,
		FlushEnabled:      true,
		WritesToCommitLog: true,
		CleanupEnabled:    true,
		RepairEnabled:     false,
		SnapshotEnabled:   true,
		RetentionOptions: myspec.RetentionOptions{
			RetentionPeriod:                     (2 * 24 * time.Hour).String(),
			BlockSize:                           (2 * time.Hour).String(),
			BufferFuture:                        (10 * time.Minute).String(),
			BufferPast:                          (10 * time.Minute).String(),
			BlockDataExpiry:                     true,
			BlockDataExpiryAfterNotAccessPeriod: (5 * time.Minute).String(),
		},
		IndexOptions: myspec.IndexOptions{
			Enabled:   true,
			BlockSize: (2 * time.Hour).String(),
		},
    ColdWritesEnabled: false,
	}

	presetOneMinuteFourtyDaysIndexed = myspec.NamespaceOptions{
		BootstrapEnabled:  true,
		FlushEnabled:      true,
		WritesToCommitLog: true,
		CleanupEnabled:    true,
		RepairEnabled:     false,
		SnapshotEnabled:   true,
		RetentionOptions: myspec.RetentionOptions{
			RetentionPeriod:                     (40 * 24 * time.Hour).String(),
			BlockSize:                           (24 * time.Hour).String(),
			BufferFuture:                        (10 * time.Minute).String(),
			BufferPast:                          (20 * time.Minute).String(),
			BlockDataExpiry:                     true,
			BlockDataExpiryAfterNotAccessPeriod: (10 * time.Minute).String(),
		},
		IndexOptions: myspec.IndexOptions{
			Enabled:   true,
			BlockSize: (24 * time.Hour).String(),
		},
    ColdWritesEnabled: false,
	}
)
