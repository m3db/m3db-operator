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

package eventer

import (
	"testing"

	"github.com/m3db/m3db-operator/pkg/client/clientset/versioned/scheme"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeFake "k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
	testEventer, err := NewEventRecorder(
		WithClient(kubeFake.NewSimpleClientset()),
		WithLogger(zap.NewNop()),
		WithNamespace("test"),
		WithComponent("testy"))

	require.NotNil(t, testEventer)
	require.NoError(t, err)
	return testEventer
}
