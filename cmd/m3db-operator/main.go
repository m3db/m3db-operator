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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/m3db/m3db-operator/pkg/controller"
	"github.com/m3db/m3db-operator/pkg/k8sops"

	"github.com/Sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
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
	flag.StringVar(&logrusLevel, "log-level", "info", "Log level for logrus")
	flag.Parse()
}

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("Operator Version: %s", appVersion)
}

func main() {
	// Identify the build
	printVersion()

	// Setup logger
	logLevel, err := logrus.ParseLevel(logrusLevel)
	if err != nil {
		fmt.Println("failed to setup log level correctly")
		os.Exit(1)
	}
	logrus.SetLevel(logLevel)

	// Create k8s client
	k8sclient, err := k8sops.New(kubeCfgFile, masterHost)
	if err != nil {
		logrus.WithError(err).Error("failed to create k8sclient")
		os.Exit(1)
	}

	// Create controller
	controller, err := controller.New(k8sclient)
	if err != nil {
		logrus.WithError(err).Error("failed to create controller")
		os.Exit(1)
	}

	// Init the controller
	if err := controller.Init(); err != nil {
		logrus.WithError(err).Error("failed to init controller")
		os.Exit(1)
	}

	// Setup main go routines and start controller
	doneChan := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	if err := controller.Start(doneChan, &wg); err != nil {
		logrus.WithError(err).Error("failed to start controler")
	}

	// Trap the INT and TERM signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			logrus.Warning("shutdown signal received")
			close(doneChan)
			wg.Wait()
			os.Exit(0)
		}
	}
}
