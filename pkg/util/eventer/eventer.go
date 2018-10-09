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
	EventReasonFailedCreate   = "FailedToCreate"
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
