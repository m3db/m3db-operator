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

package v1alpha1

import "time"

// Namespace defines an M3DB namespace or points to a preset M3DB namespace.
type Namespace struct {
	// Name is the namespace name.
	Name string `json:"name,omitempty"`

	// Preset indicates preset namespace options.
	Preset string `json:"preset,omitempty"`

	// Options points to optional custom namespace configuration.
	// +optional
	Options *NamespaceOptions `json:"options,omitempty"`
}

// RetentionOptions defines parameters for data retention.
type RetentionOptions struct {
	// RetentionPeriod controls how long data for the namespace is retained.
	RetentionPeriod time.Duration `json:"retentionPeriod,omitempty"`

	// BlockSize controls the block size for the namespace.
	BlockSize time.Duration `json:"blockSize,omitempty"`

	// BufferFuture controls how far in the future metrics can be written.
	BufferFuture time.Duration `json:"bufferFuture,omitempty"`

	// BufferPast controls how far in the past metrics can be written.
	BufferPast time.Duration `json:"bufferPast,omitempty"`

	// BlockDataExpiry controls the block expiry.
	BlockDataExpiry bool `json:"blockDataExpiry,omitempty"`

	// BlockDataExpiry controls the not after access period for expiration.
	BlockDataExpiryAfterNotAccessPeriod time.Duration `json:"blockDataExpiryAfterNotAccessPeriod,omitempty"`
}

// IndexOptions defines parameters for indexing.
type IndexOptions struct {
	// Enabled controls whether metric indexing is enabled.
	Enabled bool `json:"enabled,omitempty"`

	// BlockSize controls the index block size.
	BlockSize time.Duration `json:"blockSize,omitempty"`
}

// NamespaceOptions defines parameters for an M3DB namespace. See
// https://m3db.github.io/m3/operational_guide/namespace_configuration/ for more
// details.
type NamespaceOptions struct {
	// BootstrapEnabled control if bootstrapping is enabled.
	BootstrapEnabled bool `json:"bootstrapEnabled,omitempty"`

	// FlushEnabled controls whether flushing is enabled.
	FlushEnabled bool `json:"flushEnabled,omitempty"`

	// WritesToCommitLog controls whether commit log writes are enabled.
	WritesToCommitLog bool `json:"writesToCommitLog,omitempty"`

	// CleanupEnabled controls whether cleanups are enabled.
	CleanupEnabled bool `json:"cleanupEnabled,omitempty"`

	// RepairEnabled controls whether repairs are enabled.
	RepairEnabled bool `json:"repairEnabled,omitempty"`

	// SnapshotEnabled controls whether snapshotting is enabled.
	SnapshotEnabled bool `json:"snapshotEnabled,omitempty"`

	// RetentionOptions sets the retention parameters.
	RetentionOptions RetentionOptions `json:"retentionOptions,omitempty"`

	// IndexOptions sets the indexing parameters.
	IndexOptions IndexOptions `json:"indexOptions,omitempty"`
}
