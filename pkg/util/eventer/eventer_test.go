package eventer

import (
	"testing"

	"github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeFake "k8s.io/client-go/kubernetes/fake"
)

var (
	creator runtime.ObjectCreater = scheme.Scheme
)

func TestNormalEvent(t *testing.T) {
	testObject, err := creator.New(schema.GroupVersionKind{})
	// empty object will throw error
	assert.Error(t, err)

	testEventer := testNewEventRecorder(t)
	testEventer.NormalEvent(testObject, ReasonAdding, "this is a normal event")
}

func TestWarningEvent(t *testing.T) {
	testObject, err := creator.New(schema.GroupVersionKind{})
	// empty object will throw error
	assert.Error(t, err)

	testEventer := testNewEventRecorder(t)
	testEventer.WarningEvent(testObject, ReasonFailSync, "this is a warning event")
}

func testNewEventRecorder(t *testing.T) Poster {
	testEventer := NewEventRecorder(kubeFake.NewSimpleClientset(), zap.NewNop(), "test", "testy")
	require.NotNil(t, testEventer)
	return testEventer
}
