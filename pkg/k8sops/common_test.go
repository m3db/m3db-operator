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

package k8sops

import (
	"fmt"
	"io/ioutil"
	"testing"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func getFixture(filename string, t *testing.T) myspec.M3DBCluster {
	spec := myspec.M3DBCluster{}
	file, err := ioutil.ReadFile(fmt.Sprintf("./fixtures/%s", filename))
	if err != nil {
		t.Logf("Failed to read fixtures file: %v ", err)
		t.Fail()
	}
	err = yaml.Unmarshal(file, &spec)
	if err != nil {
		t.Logf("Unmarshal error: %v", err)
		t.Fail()
	}
	return spec
}

func TestCommon(t *testing.T) {
	k, err := newFakeK8sops()
	require.Nil(t, err)
	ssName := k.StatefulSetName("testCluster", "testIsolationGroup")
	require.Equal(t, "testCluster-testIsolationGroup-m3", ssName)
}
