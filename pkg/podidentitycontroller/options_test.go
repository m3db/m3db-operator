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

package podidentitycontroller

import (
	"testing"

	clientsetfake "github.com/m3db/m3db-operator/pkg/client/clientset/versioned/fake"
	m3dbinformers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
	"go.uber.org/zap"
)

func TestOptions(t *testing.T) {
	mc := gomock.NewController(t)
	defer mc.Finish()

	opts := &options{}

	kubeClient := kubefake.NewSimpleClientset()
	crdClient := clientsetfake.NewSimpleClientset()
	client := k8sops.NewMockK8sops(mc)
	provider := podidentity.NewMockProvider(mc)
	for _, o := range []Option{
		WithScope(tally.NoopScope),
		WithLogger(zap.NewNop()),
		WithKClient(client),
		WithCRDClient(crdClient),
		WithKubeClient(kubeClient),
		WithPodIdentityProvider(provider),
		WithKubeInformerFactory(kubeinformers.NewSharedInformerFactory(kubeClient, 0)),
		WithM3DBClusterInformerFactory(m3dbinformers.NewSharedInformerFactory(crdClient, 0)),
	} {
		assert.NotNil(t, o)
		o.execute(opts)
	}

	assert.NoError(t, opts.validate())
}
