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

package k8sops

import (
	"time"

	"github.com/Sirupsen/logrus"
	m3dboperator "github.com/m3db/m3db-operator/pkg/apis/m3dboperator"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	genclient "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"

	"k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	initContainerClusterVersionMin = []int{1, 8}
)

// K8sops defines the kube object
type K8sops struct {
	Config     *rest.Config
	CrdClient  genclient.Interface
	Kclient    kubernetes.Interface
	KubeExt    apiextensionsclient.Interface
	MasterHost string
}

// New creates a new instance of k8sops
func New(kubeCfgFile, masterHost string) (*K8sops, error) {
	crdClient, kubeClient, kubeExt, err := newKubeClient(kubeCfgFile)
	if err != nil {
		logrus.Fatalf("could not init Kubernetes client: %s", err)
	}
	return &K8sops{
		Kclient:    kubeClient,
		MasterHost: masterHost,
		CrdClient:  crdClient,
		KubeExt:    kubeExt,
	}, nil
}

func buildConfig(kubeCfgFile string) (*rest.Config, error) {
	if kubeCfgFile != "" {
		logrus.Infof("using OutOfCluster k8s config with kubeConfigFile: %s", kubeCfgFile)
		config, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)
		if err != nil {
			panic(err.Error())
		}

		return config, nil
	}

	logrus.Info("using InCluster k8s config")
	return rest.InClusterConfig()
}

func newKubeClient(kubeCfgFile string) (genclient.Interface, kubernetes.Interface, apiextensionsclient.Interface, error) {
	config, err := buildConfig(kubeCfgFile)
	if err != nil {
		panic(err)
	}

	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	kubeExtCli, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientSet, kubeClient, kubeExtCli, nil
}

// MonitorM3DBEvents watches for new or removed clusters
func (k *K8sops) MonitorM3DBEvents(stopchan chan struct{}) (<-chan *myspec.M3DBCluster, <-chan error) {
	events := make(chan *myspec.M3DBCluster)
	errc := make(chan error, 1)

	source := cache.NewListWatchFromClient(k.CrdClient.OperatorV1().RESTClient(), m3dboperator.ResourcePlural, v1.NamespaceAll, fields.Everything())
	createAddHandler := func(obj interface{}) {
		event := obj.(*myspec.M3DBCluster)
		event.Type = "ADDED"
		events <- event
	}

	createDeleteHandler := func(obj interface{}) {
		event := obj.(*myspec.M3DBCluster)
		event.Type = "DELETED"
		events <- event
	}

	updateHandler := func(old interface{}, obj interface{}) {
		event := obj.(*myspec.M3DBCluster)
		event.Type = "MODIFIED"
		events <- event
	}

	_, controller := cache.NewInformer(
		source,
		&myspec.M3DBCluster{},
		time.Minute*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    createAddHandler,
			UpdateFunc: updateHandler,
			DeleteFunc: createDeleteHandler,
		})

	go controller.Run(stopchan)

	return events, errc
}
