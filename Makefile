PROJECT_NAME := m3db-operator
OUTPUT_DIR   := out
DOCS_OUT_DIR := site
DEP_VERSION  := v0.5.0
.DEFAULT_GOAL := all

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(SELF_DIR)/.ci/common.mk

SHELL  = /bin/bash -o pipefail
GOPATH = $(shell eval $$(go env | grep GOPATH) && echo $$GOPATH)
GOBIN  ?= $(GOPATH)/bin

define LICENSE_HEADER
Copyright (c) 2019 Uber Technologies, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
endef

process_coverfile    := .ci/codecov.sh
html_report          := coverage.html
test                 := .ci/test-cover.sh
convert-test-data    := .ci/convert-test-data.sh
coverfile            := cover.out
coverage_xml         := coverage.xml
junit_xml            := junit.xml
test_log             := test.log
lint_check           := .ci/lint.sh
metalint_check       := .ci/metalint.sh
metalint_config      := .metalinter.json
metalint_exclude     := .excludemetalint
gopath_prefix        := $(GOPATH)/src
package_root         := github.com/m3db/m3db-operator
package_path         := $(gopath_prefix)/$(package_root)
tools_bin_path       := $(abspath ./_tools/bin)
combined_bin_paths   := $(tools_bin_path):$(GOBIN)
vendor_prefix        := vendor
mockgen_package      := github.com/golang/mock/mockgen
mocks_output_dir     := generated/mocks
mocks_rules_dir      := generated/mocks
auto_gen             := scripts/auto-gen.sh
GOMETALINT_VERSION   := v2.0.5

LINUX_AMD64_ENV 					:= GOOS=linux GOARCH=amd64 CGO_ENABLED=0
GO_BUILD_LDFLAGS_CMD      := $(abspath ./.ci/go-build-ldflags.sh) $(package_root)
GO_BUILD_LDFLAGS          := $(shell $(GO_BUILD_LDFLAGS_CMD))

CMDS :=        		\
	docgen       		\
	m3db-operator 	\

## Binary rules

out:
	@mkdir -p $$(pwd)/$(OUTPUT_DIR)

define CMD_RULES

.PHONY: $(CMD)
$(CMD):
	@echo "--- make $(CMD)"
	go build -ldflags '$(GO_BUILD_LDFLAGS)' -o $(OUTPUT_DIR)/$(CMD) ./cmd/$(CMD)

$(CMD)-linux-amd64:
	$(LINUX_AMD64_ENV) make $(CMD)

endef

$(foreach CMD,$(CMDS),$(eval $(CMD_RULES)))

.PHONY: bins bins-no-deps
bins: $(CMDS)
bins-no-deps: $(foreach CMD,$(CMDS),$(CMD)-no-deps)

# Target to make sure integration tests build even if we're not running them.
.PHONY: build-integration
build-integration:
	go build -tags integration ./integration/...

.PHONY: lint
lint: install-tools
	@echo "--- $@"
	PATH=$(combined_bin_paths):$(PATH) $(lint_check)

.PHONY: metalint
metalint: install-tools install-gometalinter
	@echo "--- $@"
	@(PATH=$(combined_bin_paths):$(PATH) $(metalint_check) $(metalint_config) $(metalint_exclude) && echo "metalinted successfully!") || (echo "metalinter failed" && exit 1)

.PHONY: test-xml
test-xml: test-base
	@echo "--- $@"
	go-junit-report < $(test_log) > $(junit_xml)
	gocov convert $(coverfile) | gocov-xml > $(coverage_xml)
	@$(convert-test-data) $(coverage_xml)
	@rm $(coverfile) &> /dev/null

.PHONY: test-all
test-all: clean-all install-tools verify-gen lint metalint test-all-gen bins test
	@echo "--- $@"

.PHONY: test
test: install-tools test-base
	@echo "--- $@"
	@$(tools_bin_path)/gocov convert $(coverfile) | $(tools_bin_path)/gocov report

.PHONY: test-no-deps
test-no-deps: test-base
	@echo "--- $@"
	@$(tools_bin_path)/gocov convert $(coverfile) | $(tools_bin_path)/gocov report

.PHONY: kind-create-cluster
kind-create-cluster:
	@echo "--- Starting KIND cluster"
	@./scripts/kind-create-cluster.sh

.PHONY: test-e2e
test-e2e: kind-create-cluster
	@echo "--- $@"
	PATH=$(HOME)/bin:$(PATH) $(SELF_DIR)/scripts/run_e2e_tests.sh

.PHONY: testhtml
testhtml: test-base
	@echo "--- $@"
	gocov convert $(coverfile) | gocov-html > $(html_report) && open $(html_report)
	@rm -f $(test_log) &> /dev/null

.PHONY: test-ci-unit
test-ci-unit: install-tools test-base verify-gen
	@echo "--- $@"
	$(codecov_push) $(coverfile)

.PHONY: install-tools
install-tools:
	@echo "--- $@"
	GOBIN=$(tools_bin_path) go install github.com/axw/gocov/gocov
	GOBIN=$(tools_bin_path) go install github.com/garethr/kubeval
	GOBIN=$(tools_bin_path) go install github.com/golang/mock/mockgen
	GOBIN=$(tools_bin_path) go install github.com/m3db/build-tools/linters/badtime
	GOBIN=$(tools_bin_path) go install github.com/m3db/build-tools/linters/importorder
	GOBIN=$(tools_bin_path) go install github.com/m3db/build-tools/utilities/genclean
	GOBIN=$(tools_bin_path) go install github.com/m3db/tools/update-license
	GOBIN=$(tools_bin_path) go install github.com/rakyll/statik
	GOBIN=$(tools_bin_path) go install golang.org/x/lint/golint
	GOBIN=$(tools_bin_path) go install k8s.io/kube-openapi/cmd/openapi-gen

.PHONY: install-gometalinter
install-gometalinter:
	@mkdir -p $(tools_bin_path)
	@echo "--- Installing gometalinter"
	./scripts/install-gometalinter.sh -b $(tools_bin_path) -d $(GOMETALINT_VERSION)

.PHONY: mock-gen
mock-gen: install-tools mock-gen-no-deps
	@echo "--- $@"

.PHONY: license-gen
license-gen:
	@echo "--- :apache: $@"
	@find $(SELF_DIR)/pkg/$(SUBDIR) $(SELF_DIR)/integration -name '*.go' | PATH=$(tools_bin_path):$(PATH) xargs -I{} update-license {}

.PHONY: mock-gen-no-deps
mock-gen-no-deps:
	@echo "--- $@"
	@echo generating mocks
	PATH=$(combined_bin_paths):$(PATH) PACKAGE=$(package_root) $(auto_gen) $(mocks_output_dir) $(mocks_rules_dir)

export LICENSE_HEADER
.PHONY: asset-gen
asset-gen:
	@echo "--- $@"
	@echo generating assets
	PATH=$(tools_bin_path):$(PATH) statik -src $(SELF_DIR)/assets -dest $(SELF_DIR)/pkg/ -p assets -f -m -c "$$LICENSE_HEADER"

# NB(schallert): order matters -- we want license generation after all else.
.PHONY: all-gen
all-gen: mock-gen kubernetes-gen asset-gen helm-bundle docs-api-gen license-gen

# Ensure base commit had up-to-date generated artifacts
.PHONY: test-all-gen
test-all-gen: all-gen
	@echo "--- :git: verifying HEAD up-to-date with generated code"
	@test "$(shell git diff --exit-code --shortstat 2>/dev/null)" = "" || (git diff --exit-code && echo "Check git status, there are dirty files" && exit 1)
	@test "$(shell git status --exit-code --porcelain 2>/dev/null | grep "^??")" = "" || (git status --exit-code --porcelain && echo "Check git status, there are untracked files" && exit 1)
	@echo "--- end codegen verification"

.PHONY: clean ## Clean cleans all artifacts we may generate.
clean:
	@rm -f *.html *.xml *.out *.test
	@rm -rf $(OUTPUT_DIR)

.PHONY: clean-all
clean-all: clean ## Clean-all cleans all build dependencies.
	@echo "--- $@"
	@go clean
	@rm -rf vendor/
	@rm -rf _tools/

.PHONY: all
all: clean-all kubernetes-gen lint metalint test-ci-unit bins
	@echo "$@ successfully finished"

.PHONY: kubernetes-gen
kubernetes-gen: install-tools ## Generate boilerplate code for kubernetes packages
	@echo "--- $@"
	## pull in correct version of script
	go mod vendor
	@GOPATH=$(GOPATH) PATH=$(tools_bin_path):$(PATH) ./hack/update-generated.sh

.PHONY: verify-gen
verify-gen: ## Ensure all codegen is up to date
	@GOPATH=$(GOPATH) PATH=$(tools_bin_path):$(PATH) ./hack/verify-generated.sh

.PHONY: build-docker
build-docker: ## Build m3db-operator docker image with go binary
	@echo "--- $@"
	@./build/build-docker.sh

.PHONY: helm-bundle-no-deps
helm-bundle-no-deps:
	@echo "--- $@"
	@helm template --namespace default helm/m3db-operator > bundle.yaml
	@PATH=$(tools_bin_path):$(PATH) kubeval -v=1.12.0 bundle.yaml

.PHONY: helm-bundle
helm-bundle: install-tools helm-bundle-no-deps

.PHONY: publish-helm-charts
publish-helm-charts: ## pushes a new version of the helm chart
	@echo "+ $@"
	./build/package-helm-charts.sh

## Documentation

.PHONY: docs-clean
docs-clean:
	mkdir -p $(DOCS_OUT_DIR)
	rm -rf $(DOCS_OUT_DIR)/*

.PHONY: docs-container
docs-container:
	docker build -t m3db-docs -f docs/Dockerfile docs

.PHONY: docs-build
docs-build: docs-clean docs-container
	docker run -v $(PWD):/m3db --rm m3db-docs "mkdocs build -e docs/theme -t material"

.PHONY: docs-serve
docs-serve: docs-clean docs-container
	docker run -v $(PWD):/m3db -p 8000:8000 -it --rm m3db-docs "mkdocs serve -e docs/theme -t material -a 0.0.0.0:8000"

.PHONY: docs-api-gen-no-deps
docs-api-gen-no-deps:
	$(SELF_DIR)/out/docgen api pkg/apis/m3dboperator/v1alpha1/cluster.go pkg/apis/m3dboperator/v1alpha1/namespace.go pkg/apis/m3dboperator/v1alpha1/pod_identity.go > $(SELF_DIR)/docs/api.md

.PHONY: docs-api-gen
docs-api-gen: docgen docs-api-gen-no-deps
