// +build integration

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

package harness

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func exitErr() {
	os.Exit(1)
}

func buildLogger(flags *Flags) *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

	logger, err := cfg.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building logger: %v", err)
		exitErr()
	}

	return logger
}

func homeKubeConfig() string {
	home := os.Getenv("HOME")
	if home == "" {
		fmt.Fprintf(os.Stderr, "$HOME cannot be empty if not setting kubeconfig path")
		exitErr()
	}

	return filepath.Join(home, ".kube", "config")
}

// Flags define parameters for the test.
type Flags struct {
	KubeConfig          string
	Verbose             bool
	SkipCreateNamespace bool
	SkipDeleteNamespace bool
	SkipCreateRBAC      bool
	SkipDeleteRBAC      bool
}

// RunSuite runs the test suite.
func RunSuite(m *testing.M) {
	flags := Flags{}
	flag.StringVar(&flags.KubeConfig, "kubeconfig", "", "Path to kubeconfig (default $HOME/.kube/config)")
	flag.BoolVar(&flags.Verbose, "debug", false, "enable debug logging")
	flag.BoolVar(&flags.SkipCreateNamespace, "skip-create-ns", false, "skip namespace creation")
	flag.BoolVar(&flags.SkipDeleteNamespace, "skip-delete-ns", false, "skip namespace deletion")
	flag.BoolVar(&flags.SkipCreateRBAC, "skip-create-rbac", false, "skip RBAC resource creation")
	flag.BoolVar(&flags.SkipDeleteRBAC, "skip-delete-rbac", false, "skip RBAC resource deletion")
	flag.Parse()

	if flags.KubeConfig == "" {
		flags.KubeConfig = homeKubeConfig()
	}

	logger := buildLogger(nil)
	logger.Info("setting up test suite")

	opts := &suiteOpts{
		flags:  flags,
		logger: logger,
	}

	harness, err := buildSuite(opts)
	if err != nil {
		logger.Fatal("error building suite", zap.Error(err))
	}

	ctx := context.Background()
	if err := harness.setupSuite(ctx); err != nil {
		logger.Fatal("error setting up suite", zap.Error(err))
	}

	Global = harness

	code := m.Run()

	if err := harness.teardownSuite(ctx); err != nil {
		logger.Fatal("error tearing down suite")
	}

	os.Exit(code)
}
