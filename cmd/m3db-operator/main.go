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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/m3db/m3/src/query/util/logging"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	"github.com/m3db/m3db-operator/pkg/controller"
	"github.com/m3db/m3db-operator/pkg/k8sops"

	"go.uber.org/zap"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	appVersion  = "0.0.1"
	kubeCfgFile string
	masterHost  string
	logrusLevel string
)

func init() {
	flag.StringVar(&kubeCfgFile, "kubecfg-file", "", "Location of kubecfg file for access to kubernetes master service; --kube_master_url overrides the URL part of this; if neither this nor --kube_master_url are provided, defaults to service account tokens")
	flag.StringVar(&masterHost, "masterhost", "http://127.0.0.1:8001", "Full url to k8s api server")
	flag.Parse()
}

func main() {

	logging.InitWithCores(nil)
	ctx := context.Background()
	logger := logging.WithContext(ctx)
	defer logger.Sync()

	logger.Info("Go", zap.Any("VERSION", runtime.Version()))
	logger.Info("Go", zap.Any("OS", runtime.GOOS), zap.Any("ARCH", runtime.GOARCH))
	logger.Info("Operator", zap.String("version", appVersion))

	// Create k8s clients
	crdClient, kubeClient, kubeExt, err := newKubeClient(logger, kubeCfgFile)
	if err != nil {
		logger.Fatal("failed to create k8s clients", zap.Error(err))
	}

	// Create k8sops client
	k8sclient, err := k8sops.New(
		masterHost,
		k8sops.WithLogger(logger),
		k8sops.WithCRDClient(crdClient),
		k8sops.WithKClient(kubeClient),
		k8sops.WithExtClient(kubeExt))
	if err != nil {
		logger.Fatal("failed to create k8sclient", zap.Error(err))
	}

	// Create controller
	controller, err := controller.New(logger, k8sclient)
	if err != nil {
		logger.Fatal("failed to create controller", zap.Error(err))
	}

	// Init the controller
	if err := controller.Init(); err != nil {
		logger.Fatal("failed to init controller", zap.Error(err))
	}

	// Setup main go routines and start controller
	var wg sync.WaitGroup
	wg.Add(1)
	if err := controller.Start(&wg); err != nil {
		logger.Fatal("failed to start controler", zap.Error(err))
	}

	// Trap the INT and TERM signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			logger.Warn("shutdown signal received")
			controller.Stop()
			wg.Wait()
			os.Exit(0)
		}
	}
}

func buildConfig(logger *zap.Logger, kubeCfgFile string) (*rest.Config, error) {
	if kubeCfgFile != "" {
		logger.Info("using OutOfCluster k8s config", zap.String("kubeFile", kubeCfgFile))
		config, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)
		if err != nil {
			return nil, err
		}
		return config, nil
	}
	logger.Info("using InCluster k8s config")
	return rest.InClusterConfig()
}

func newKubeClient(logger *zap.Logger, kubeCfgFile string) (clientset.Interface, kubernetes.Interface, apiextensionsclient.Interface, error) {
	config, err := buildConfig(logger, kubeCfgFile)
	if err != nil {
		return nil, nil, nil, err
	}

	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	kubeExtCli, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	return clientSet, kubeClient, kubeExtCli, nil
}
