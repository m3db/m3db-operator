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
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// Types of kubernetes emitted events
const (

	// Add events
	EventReasonAdding        = "Adding"
	EventReasonFailedToAdd   = "FailedToAdd"
	EventReasonSuccessfulAdd = "SuccessfulAdd"

	// Delete events
	EventReasonDeleting         = "Deleting"
	EventReasonFailedToDelete   = "FailedToDelete"
	EventReasonSuccessfulDelete = "SuccessfulDelete"

	// Delete events
	EventReasonCreating         = "Creating"
	EventReasonFailedCreate     = "FailedToCreate"
	EventReasonSuccessfulCreate = "SuccessfulCreate"

	// Update events
	EventReasonUpdating         = "Updating"
	EventReasonFailedToUpdate   = "FailedToUpdate"
	EventReasonSuccessfulUpdate = "SuccessfulUpdate"

	// Sync events
	EventReasonSyncing     = "Syncing"
	EventReasonSuccessSync = "FailedToSync"
	EventReasonFailSync    = "SuccessfulSync"

	// Misc events
	EventReasonLongerThanUsual = "TimeLongerThanUsual"
	EventReasonUnknown         = "Unknown"
)

// NewEventRecorder creates a new recorder to emit kubernetes events.
func NewEventRecorder(
	kubeClient kubernetes.Interface,
	logger *zap.Logger,
	component string) record.EventRecorder {

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(logger.Sugar().Infof)
	broadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events("")})

	return broadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: component})
}

// PostNormalEvent posts an event of expected healthy behavior
func PostNormalEvent(recorder record.EventRecorder, object runtime.Object, reason, message string) {
	recorder.Eventf(object,
		corev1.EventTypeNormal,
		reason,
		message)
}

// PostWarningEvent post an event of type errors or unexpectled possibly unhealthy behavior
func PostWarningEvent(recorder record.EventRecorder, object runtime.Object, reason, message string, args ...interface{}) {
	recorder.Eventf(object,
		corev1.EventTypeWarning,
		reason,
		message,
		args)
}
