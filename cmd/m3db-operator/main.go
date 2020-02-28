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
	"os"
	"os/signal"
	"syscall"
	"time"

	// No-op import to register assets.
	_ "github.com/m3db/m3db-operator/pkg/assets"
	clientset "github.com/m3db/m3db-operator/pkg/client/clientset/versioned"
	informers "github.com/m3db/m3db-operator/pkg/client/informers/externalversions"
	"github.com/m3db/m3db-operator/pkg/controller"
	"github.com/m3db/m3db-operator/pkg/k8sops/m3db"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	"github.com/m3db/m3x/instrument"

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
	_informerSyncDuration = time.Minute
)

var (
	_kubeCfgFile         string
	_masterURL           string
	_operatorName        = "m3db_operator"
	_metricsPath         = "/metrics"
	_metricsPort         = ":8080"
	_useProxy            bool
	_debugLog            bool
	_develLog            bool
	_humanTime           bool
	_manageCRD           bool
	_enableCRDValidation bool
	_namespace           string
)

func init() {
	flag.StringVar(&_kubeCfgFile, "kubecfg-file", "", "Location of kubecfg file for access to kubernetes master service; --kube_master_url overrides the URL part of this; if neither this nor --kube_master_url are provided, defaults to service account tokens")
	flag.StringVar(&_masterURL, "masterhost", "http://127.0.0.1:8001", "Full url to k8s api server")
	flag.BoolVar(&_debugLog, "debug", false, "enable debug logging")
	flag.BoolVar(&_develLog, "devel", false, "enable development logging mode")
	flag.BoolVar(&_humanTime, "human-time", false, "print human-friendly timestamps")
	flag.BoolVar(&_useProxy, "proxy", false, "use kubectl proxy for cluster communication")
	flag.BoolVar(&_manageCRD, "manage-crd", true, "create and update the operator's CRD specs")
	// Disabled by default until openAPI validation is more tested.
	flag.BoolVar(&_enableCRDValidation, "enable-crd-validation", false, "enable openAPI validation of the CR")
	flag.StringVar(&_namespace, "namespace", "all", "specify a specific namespace to watch. Or specify all to watch all namespaces")
	flag.Parse()
}

func main() {
	var cfg zap.Config
	if _develLog {
		cfg = zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	} else {
		cfg = zap.NewProductionConfig()
	}
	if _debugLog {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}
	if _humanTime {
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	cfg.DisableStacktrace = true
	cfg.DisableCaller = true
	logger, err := cfg.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building logger: %v", err)
		os.Exit(1)
	}

	defer logger.Sync()

	// Setup telemetry
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}
	tags := map[string]string{
		"environment": env,
	}

	promCfg := promreporter.Configuration{
		HandlerPath:   _metricsPath,
		ListenAddress: _metricsPort,
		TimerType:     "summary",
	}

	r, err := promCfg.NewReporter(promreporter.ConfigurationOptions{
		OnError: func(err error) {
			if err != nil {
				logger.Error("prometheus reporter error", zap.Error(err))
			}
		},
	})
	if err != nil {
		logger.Fatal("error constructing prom reporter", zap.Error(err))
	}

	scope, closer := tally.NewRootScope(tally.ScopeOptions{
		Prefix:          _operatorName,
		Tags:            tags,
		CachedReporter:  r,
		SanitizeOptions: &promreporter.DefaultSanitizerOpts,
		Separator:       promreporter.DefaultSeparator,
	}, 1*time.Second)
	defer closer.Close()

	iopts := instrument.NewOptions().
		SetMetricsScope(scope)
	buildReporter := instrument.NewBuildReporter(iopts)
	if err := buildReporter.Start(); err != nil {
		logger.Fatal("unable to start build reporter", zap.Error(err))
	}
	defer buildReporter.Stop()

	// Create k8s clients
	crdClient, kubeClient, kubeExt, err := newKubeClient(logger, _masterURL, _kubeCfgFile)
	if err != nil {
		logger.Fatal("failed to create k8s clients", zap.Error(err))
	}

	// Create k8sops client
	k8sclient, err := m3db.New(
		m3db.WithLogger(logger),
		m3db.WithCRDClient(crdClient),
		m3db.WithKClient(kubeClient),
		m3db.WithExtClient(kubeExt))
	if err != nil {
		logger.Fatal("failed to create k8sclient", zap.Error(err))
	}

	stopCh := make(chan struct{})
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, _informerSyncDuration)
	if _namespace != "all" {
		kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, _informerSyncDuration,kubeinformers.WithNamespace(_namespace))
	}
	nodeLister := kubeInformerFactory.Core().V1().Nodes().Lister()
	m3dbClusterInformerFactory := informers.NewSharedInformerFactory(crdClient, _informerSyncDuration)
	if _namespace != "all" {
		m3dbClusterInformerFactory = informers.NewSharedInformerFactoryWithOptions(crdClient, _informerSyncDuration, informers.WithNamespace("m3"))
	}
	clusterLogger := logger.With(zap.String("controller", "m3db-cluster-controller"))
	idLogger := logger.With(zap.String("component", "pod-identity-provider"))
	idProvider, err := podidentity.NewProvider(
		podidentity.WithLogger(idLogger),
		podidentity.WithNodeLister(nodeLister),
	)
	if err != nil {
		logger.Fatal("failed to create ID provider", zap.Error(err))
	}

	config := controller.Configuration{
		ManageCRD:        _manageCRD,
		EnableValidation: _enableCRDValidation,
	}

	opts := []controller.Option{
		controller.WithConfig(config),
		controller.WithKubeInformerFactory(kubeInformerFactory),
		controller.WithM3DBClusterInformerFactory(m3dbClusterInformerFactory),
		controller.WithPodIdentityProvider(idProvider),
		controller.WithLogger(clusterLogger),
		controller.WithKClient(k8sclient),
		controller.WithCRDClient(crdClient),
		controller.WithKubeClient(kubeClient),
		controller.WithScope(scope),
	}

	// Override coordinator addr (i.e. running out-of-cluster and port-forwarding)
	if _useProxy {
		opts = append(opts, controller.WithKubectlProxy(true))
	}

	// Create controller
	controller, err := controller.NewM3DBController(opts...)
	if err != nil {
		logger.Fatal("failed to create controller", zap.Error(err))
	}

	go kubeInformerFactory.Start(stopCh)
	go m3dbClusterInformerFactory.Start(stopCh)

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
