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

package eventer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"go.uber.org/zap"
)

// Types of kubernetes emitted events
const (

	// Add events
	ReasonAdding        = "Adding"
	ReasonFailedToAdd   = "FailedToAdd"
	ReasonSuccessfulAdd = "SuccessfulAdd"

	// Delete events
	ReasonDeleting         = "Deleting"
	ReasonFailedToDelete   = "FailedToDelete"
	ReasonSuccessfulDelete = "SuccessfulDelete"

	// Create events
	ReasonCreating         = "Creating"
	ReasonFailedCreate     = "FailedToCreate"
	ReasonSuccessfulCreate = "SuccessfulCreate"

	// Update events
	ReasonUpdating         = "Updating"
	ReasonFailedToUpdate   = "FailedToUpdate"
	ReasonSuccessfulUpdate = "SuccessfulUpdate"

	// Sync events
	ReasonSyncing     = "Syncing"
	ReasonSuccessSync = "FailedToSync"
	ReasonFailSync    = "SuccessfulSync"

	// Misc events
	ReasonLongerThanUsual = "TimeLongerThanUsual"
	ReasonUnknown         = "Unknown"
)

// Poster posts events accordingly to kind of behavior
type Poster interface {
	NormalEvent(object runtime.Object, reason, message string, args ...interface{})
	WarningEvent(object runtime.Object, reason, message string, args ...interface{})
}

type eventer struct {
	recorder record.EventRecorder

	client    kubernetes.Interface
	logger    *zap.Logger
	namespace string
	component string
}

// NewEventRecorder creates a new recorder to emit kubernetes events.
func NewEventRecorder(eventerOpts ...Option) (Poster, error) {

	opts := &options{}
	for _, o := range eventerOpts {
		o.execute(opts)
	}

	if opts.kubeClient == nil {
		return nil, fmt.Errorf("kubeClient must not be nil")
	}

	e := &eventer{
		client:    opts.kubeClient,
		logger:    opts.logger,
		namespace: opts.namespace,
		component: opts.component,
	}

	if e.logger == nil {
		e.logger = zap.NewNop()
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(e.logger.Sugar().Infof)
	broadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: e.client.CoreV1().Events(e.namespace)})

	e.recorder = broadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: e.component})

	return e, nil
}

// NormalEvent posts an event of expected healthy behavior
func (e *eventer) NormalEvent(object runtime.Object, reason, message string, args ...interface{}) {
	e.recorder.Eventf(object,
		corev1.EventTypeNormal,
		reason,
		message,
		args)
}

// WarningEvent post an event of type errors or unexpectled possibly unhealthy behavior
func (e *eventer) WarningEvent(object runtime.Object, reason, message string, args ...interface{}) {
	e.recorder.Eventf(object,
		corev1.EventTypeWarning,
		reason,
		message,
		args)
}
