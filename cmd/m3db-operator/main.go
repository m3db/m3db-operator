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
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	informers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/controller"
	"github.com/m3db/m3db-operator/pkg/k8sops"
	"github.com/m3db/m3db-operator/pkg/m3admin"
	"github.com/m3db/m3db-operator/pkg/m3admin/namespace"
	"github.com/m3db/m3db-operator/pkg/m3admin/placement"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/uber-go/tally"
	promreporter "github.com/uber-go/tally/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// Informers will resync on this interval
	_informerSyncDuration = 30 * time.Second
)

var (
	_appVersion      = "0.0.1"
	_kubeCfgFile     string
	_masterURL       string
	_coordinatorAddr string
	_operatorName    = "m3db_operator"
	_metricsPath     = "/metrics"
	_metricsPort     = ":8080"
	_debugLog        bool
	_develLog        bool
)

func init() {
	flag.StringVar(&_kubeCfgFile, "kubecfg-file", "", "Location of kubecfg file for access to kubernetes master service; --kube_master_url overrides the URL part of this; if neither this nor --kube_master_url are provided, defaults to service account tokens")
	flag.StringVar(&_masterURL, "masterhost", "http://127.0.0.1:8001", "Full url to k8s api server")
	flag.StringVar(&_coordinatorAddr, "coordinator", "", "override coordinator address if running out-of-cluster")
	flag.BoolVar(&_debugLog, "debug", false, "enable debug logging")
	flag.BoolVar(&_develLog, "devel", false, "enable development logging mode")
	flag.Parse()
}

func main() {
	var cfg zap.Config
	if _develLog {
		cfg = zap.NewDevelopmentConfig()
		cfg.DisableStacktrace = true
		cfg.DisableCaller = true
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	} else {
		cfg = zap.NewProductionConfig()
	}
	if _debugLog {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}
	logger, err := cfg.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building logger: %v", err)
		os.Exit(1)
	}

	defer logger.Sync()

	logger.Info("Go", zap.Any("VERSION", runtime.Version()))
	logger.Info("Go", zap.Any("OS", runtime.GOOS), zap.Any("ARCH", runtime.GOARCH))
	logger.Info("Operator", zap.String("version", _appVersion))

	// Setup telemetry
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}
	tags := map[string]string{
		"environment": env,
	}
	r := promreporter.NewReporter(promreporter.Options{})
	scope, closer := tally.NewRootScope(tally.ScopeOptions{
		Prefix:         _operatorName,
		Tags:           tags,
		CachedReporter: r,
		Separator:      promreporter.DefaultSeparator,
	}, 1*time.Second)
	defer closer.Close()

	// Serve the metrics
	go func() {
		http.Handle(_metricsPath, r.HTTPHandler())
		http.ListenAndServe(_metricsPort, nil)
	}()

	// Create k8s clients
	crdClient, kubeClient, kubeExt, err := newKubeClient(logger, _masterURL, _kubeCfgFile)
	if err != nil {
		logger.Fatal("failed to create k8s clients", zap.Error(err))
	}

	// Create k8sops client
	k8sclient, err := k8sops.New(
		k8sops.WithLogger(logger),
		k8sops.WithCRDClient(crdClient),
		k8sops.WithKClient(kubeClient),
		k8sops.WithExtClient(kubeExt))
	if err != nil {
		logger.Fatal("failed to create k8sclient", zap.Error(err))
	}

	stopCh := make(chan struct{})

	// TODO(schallert): move these to k8sops client once we're confident we want
	// them
	//
	// TODO(schallert): not sure if we need podinformers as well or if
	// abstractions of statefulsets will do
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, _informerSyncDuration)
	m3dbClusterInformerFactory := informers.NewSharedInformerFactory(crdClient, _informerSyncDuration)

	opts := []controller.Option{
		controller.WithKubeInformerFactory(kubeInformerFactory),
		controller.WithM3DBClusterInformerFactory(m3dbClusterInformerFactory),
		controller.WithLogger(logger),
		controller.WithKClient(k8sclient),
		controller.WithCRDClient(crdClient),
		controller.WithKubeClient(kubeClient),
		controller.WithScope(scope),
	}

	// Override coordinator addr (i.e. running out-of-cluster and port-forwarding)
	if _coordinatorAddr != "" {
		m3adminClient := m3admin.NewClient(m3admin.WithLogger(logger))
		pc, err := placement.NewClient(
			placement.WithLogger(logger),
			placement.WithURL(_coordinatorAddr),
			placement.WithClient(m3adminClient),
		)
		if err != nil {
			logger.Fatal(err.Error())
		}
		nc, err := namespace.NewClient(namespace.WithLogger(logger), namespace.WithURL(_coordinatorAddr))
		if err != nil {
			logger.Fatal(err.Error())
		}

		opts = append(opts,
			controller.WithPlacementClient(pc),
			controller.WithNamespaceClient(nc),
		)
	}

	// Create controller
	controller, err := controller.New(opts...)
	if err != nil {
		logger.Fatal("failed to create controller", zap.Error(err))
	}

	go kubeInformerFactory.Start(stopCh)
	go m3dbClusterInformerFactory.Start(stopCh)

	// Init the controller
	if err := controller.Init(); err != nil {
		logger.Fatal("failed to init controller", zap.Error(err))
	}

	// Trap the INT and TERM signals
	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChan
		logger.Warn("received shutdown signal, sending to workers")
		close(stopCh)
		<-signalChan
		logger.Warn("received second signal, exiting immediately")
		os.Exit(1)
	}()

	if err := controller.Run(2, stopCh); err != nil {
		logger.Fatal("error running controller", zap.Error(err))
	}
}

func buildConfig(logger *zap.Logger, masterURL, kubeCfgFile string) (*rest.Config, error) {
	if kubeCfgFile != "" {
		logger.Info("using OutOfCluster k8s config", zap.String("kubeFile", kubeCfgFile))
		config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeCfgFile)
		if err != nil {
			return nil, err
		}
		return config, nil
	}
	logger.Info("using InCluster k8s config")
	return rest.InClusterConfig()
}

func newKubeClient(logger *zap.Logger, masterURL, kubeCfgFile string) (clientset.Interface, kubernetes.Interface, apiextensionsclient.Interface, error) {
	config, err := buildConfig(logger, masterURL, kubeCfgFile)
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
