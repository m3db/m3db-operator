//go:build tools
// +build tools

package tools

import (
	_ "github.com/axw/gocov/gocov"
	_ "github.com/cirocosta/grafana-sync"
	_ "github.com/fossas/fossa-cli/cmd/fossa"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/m3db/build-tools/linters/badtime"
	_ "github.com/m3db/build-tools/linters/importorder"
	_ "github.com/m3db/build-tools/utilities/genclean"
	_ "github.com/m3db/tools/update-license"
	_ "github.com/rakyll/statik"
	_ "golang.org/x/lint/golint"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
