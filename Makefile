PROJECT_NAME := m3db-operator
REPO_PATH    := github.com/m3db/m3db-operator
BUILD_PATH   := $(REPO_PATH)/cmd/$(PROJECT_NAME)
OUTPUT_DIR   := out
DEP_VERSION  := v0.5.0
BUILD_SETTINGS := GOOS=linux GOARCH=amd64 CGO_ENABLED=0
ifeq ($(shell uname), Darwin)
	BUILD_SETTINGS := GOOS=darwin GOARCH=amd64 CGO_ENABLED=0
endif
.DEFAULT_GOAL := all

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(SELF_DIR)/.ci/common.mk

SHELL=/bin/bash -o pipefail

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
retool_bin_path      := $(package_path)/_tools/bin
retool_package       := github.com/twitchtv/retool
vendor_prefix        := vendor
mockgen_package      := github.com/golang/mock/mockgen
mocks_output_dir     := generated/mocks
mocks_rules_dir      := generated/mocks
auto_gen             := scripts/auto-gen.sh
GOMETALINT_VERSION   := v2.0.5

LINUX_AMD64_ENV := GOOS=linux GOARCH=amd64 CGO_ENABLED=0

.PHONY: lint
lint: install-codegen-tools
	@echo "--- $@"
	PATH=$(retool_bin_path):$(PATH) $(lint_check)

.PHONY: metalint
metalint: install-codegen-tools dep-ensure install-gometalinter
	@echo "--- $@"
	@(PATH=$(retool_bin_path):$(PATH) $(metalint_check) $(metalint_config) $(metalint_exclude) && echo "metalinted successfully!") || (echo "metalinter failed" && exit 1)

.PHONY: test-xml
test-xml: test-base
	@echo "--- $@"
	go-junit-report < $(test_log) > $(junit_xml)
	gocov convert $(coverfile) | gocov-xml > $(coverage_xml)
	@$(convert-test-data) $(coverage_xml)
	@rm $(coverfile) &> /dev/null

.PHONY: test
test: test-base
	@echo "--- $@"
	gocov convert $(coverfile) | gocov report

.PHONY: testhtml
testhtml: test-base
	@echo "--- $@"
	gocov convert $(coverfile) | gocov-html > $(html_report) && open $(html_report)
	@rm -f $(test_log) &> /dev/null

.PHONY: test-ci-unit
test-ci-unit: install-ci-tools test-base verify-gen
	@echo "--- $@"
	$(codecov_push) $(coverfile)

.PHONY: install-ci-tools
install-ci-tools: install-codegen-tools dep-ensure install-mockgen
	@echo "--- $@"
	@which gocov > /dev/null || go get github.com/axw/gocov/gocov

# NB(prateek): cannot use retool for mock-gen, as mock-gen reflection mode requires
# it's full source code be present in the GOPATH at runtime.
.PHONY: install-mockgen
install-mockgen:
	@echo "--- $@"
	@which mockgen >/dev/null || (                                                     \
		rm -rf $(gopath_prefix)/$(mockgen_package)                                    && \
		mkdir -p $(shell dirname $(gopath_prefix)/$(mockgen_package))                 && \
		cp -r $(vendor_prefix)/$(mockgen_package) $(gopath_prefix)/$(mockgen_package) && \
		go install $(mockgen_package)                                                    \
	)

.PHONY: install-retool
install-retool:
	@which retool >/dev/null || go get $(retool_package)

.PHONY: install-codegen-tools
install-codegen-tools: install-retool
	@echo "--- Installing retool dependencies"
	@retool sync >/dev/null 2>/dev/null
	@retool build >/dev/null 2>/dev/null

.PHONY: install-gometalinter
install-gometalinter:
	@mkdir -p $(retool_bin_path)
	@echo "--- Installing gometalinter"
	./scripts/install-gometalinter.sh -b $(retool_bin_path) -d $(GOMETALINT_VERSION)

.PHONY: install-proto-bin
install-proto-bin: install-codegen-tools
	@echo "--- $@, Installing protobuf binaries"
	@echo Note: the protobuf compiler v3.0.0 can be downloaded from https://github.com/google/protobuf/releases or built from source at https://github.com/google/protobuf.
	go install $(package_root)/$(vendor_prefix)/$(protoc_go_package)

.PHONY: mock-gen
mock-gen: install-ci-tools mock-gen-no-deps
	@echo "--- $@"

.PHONY: license-gen
license-gen:
	@echo "--- :apache: $@"
	@find $(SELF_DIR)/pkg/$(SUBDIR) -name '*.go' | PATH=$(retool_bin_path):$(PATH) xargs -I{} update-license {}

.PHONY: mock-gen-no-deps
mock-gen-no-deps:
	@echo "--- $@"
	@echo generating mocks
	PATH=$(retool_bin_path):$(PATH) PACKAGE=$(package_root) $(auto_gen) $(mocks_output_dir) $(mocks_rules_dir)

.PHONY: all-gen
all-gen: mock-gen kubernetes-gen license-gen

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
all: clean-all kubernetes-gen lint metalint test-ci-unit
	@echo "$@ successfully finished"

.PHONY: dep-ensure
dep-ensure: install-codegen-tools ## Run dep ensure to generate vendor directory
	@echo "--- $@"
	PATH=$(retool_bin_path):$(PATH) dep ensure

.PHONY: kubernetes-gen
kubernetes-gen: dep-ensure ## Generate boilerplate code for kubernetes packages
	@echo "--- $@"
	@./hack/update-generated.sh

.PHONY: verify-gen
verify-gen: dep-ensure ## Ensure all codegen is up to date
	@./hack/verify-generated.sh

.PHONY: build-bin
build-bin: out ## Build m3db-operator binary
	@echo "--- $@"
	@which go > /dev/null || (echo "error: golang needs to be installed" && exit 1)
	@echo "building $(PROJECT_NAME)..."
	$(BUILD_SETTINGS) go build -o $(OUTPUT_DIR)/$(PROJECT_NAME) $(BUILD_PATH)

.PHONY: build-docker
build-docker: ## Build m3db-operator docker image with go binary
	@echo "--- $@"
	@./build/build-docker.sh

.PHONY: publish-helm-charts
publish-helm-charts: ## pushes a new version of the helm chart
	@echo "+ $@"
	./build/package-helm-charts.sh

out:
	@mkdir -p $$(pwd)/$(OUTPUT_DIR)
