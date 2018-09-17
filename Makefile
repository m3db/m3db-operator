PROJECT_NAME := m3db-operator
REPO_PATH    := github.com/m3db/m3db-operator
BUILD_PATH   := $(REPO_PATH)/cmd/$(PROJECT_NAME)
OUTPUT_DIR   := _output
BUILD_SETTINGS := GOOS=linux GOARCH=amd64 CGO_ENABLED=0 
ifeq ($(shell uname), Darwin)
	BUILD_SETTINGS := GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 
endif
.DEFAULT_GOAL := all

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(SELF_DIR)/.ci/common.mk

SHELL=/bin/bash -o pipefail

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
package_root         := github.com/m3db/m3db-operator
gopath_prefix        := $(GOPATH)/src
vendor_prefix        := vendor
mockgen_package      := github.com/golang/mock/mockgen
mocks_output_dir     := generated/mocks/mocks
mocks_rules_dir      := generated/mocks
auto_gen             := .ci/auto-gen.sh
license_dir          := .ci/uber-licence
license_node_modules := $(license_dir)/node_modules

BUILD           := $(abspath ./bin)
LINUX_AMD64_ENV := GOOS=linux GOARCH=amd64 CGO_ENABLED=0

.PHONY: setup
setup:
	mkdir -p $(BUILD)

.PHONY: lint
lint:
	@which golint > /dev/null || go get -u github.com/golang/lint/golint
	$(VENDOR_ENV) $(lint_check)

.PHONY: metalint
metalint: install-metalinter install-linter-badtime
	@($(metalint_check) $(metalint_config) $(metalint_exclude) && echo "metalinted successfully!") || (echo "metalinter failed" && exit 1)

.PHONY: test-internal
test-internal:
	@which go-junit-report > /dev/null || go get -u github.com/sectioneight/go-junit-report
	@$(VENDOR_ENV) $(test) $(coverfile) | tee $(test_log)

.PHONY: test-xml
test-xml: test-internal
	go-junit-report < $(test_log) > $(junit_xml)
	gocov convert $(coverfile) | gocov-xml > $(coverage_xml)
	@$(convert-test-data) $(coverage_xml)
	@rm $(coverfile) &> /dev/null

.PHONY: test
test: test-internal
	gocov convert $(coverfile) | gocov report

.PHONY: testhtml
testhtml: test-internal
	gocov convert $(coverfile) | gocov-html > $(html_report) && open $(html_report)
	@rm -f $(test_log) &> /dev/null

.PHONY: test-ci-unit
test-ci-unit: test-internal
	@which goveralls > /dev/null || go get -u -f github.com/mattn/goveralls
	goveralls -coverprofile=$(coverfile) -service=travis-ci || echo -e "\x1b[31mCoveralls failed\x1b[m"

.PHONY: install-mockgen
install-mockgen: install-vendor-dep
	@which goveralls > /dev/null || (go get github.com/golang/mock/gomock && \
		go install github.com/golang/mock/mockgen)

.PHONY: install-licence-bin
install-license-bin: install-vendor-dep
	@echo Installing node modules
	[ -d $(license_node_modules) ] || (cd $(license_dir) && npm install)

.PHONY: install-proto-bin
install-proto-bin: install-vendor-dep
	@echo Installing protobuf binaries
	@echo Note: the protobuf compiler v3.0.0 can be downloaded from https://github.com/google/protobuf/releases or built from source at https://github.com/google/protobuf.
	go install $(package_root)/$(vendor_prefix)/$(protoc_go_package)

.PHONY: mock-gen
mock-gen: install-mockgen install-license-bin install-util-mockclean
	@echo Generating mocks
	PACKAGE=$(package_root) $(auto_gen) $(mocks_output_dir) $(mocks_rules_dir)

.PHONY: mock-gen-deps
mock-gen-no-deps:
	@echo Generating mocks
	PACKAGE=$(package_root) $(auto_gen) $(mocks_output_dir) $(mocks_rules_dir)

.PHONY: all-gen
all-gen: mock-gen code-gen

.PHONY: clean
clean:
	@rm -f *.html *.xml *.out *.test
	@go clean
	@rm -f build/output/bin/m3db-operator

.PHONY: all
all: clean code-gen lint metalint test-ci-unit
	@echo make all successfully finished

.PHONY: dep-install
dep-install: ## Ensure dep is installed
	@which dep > /dev/null || ./build/install-dep.sh

.PHONY: dep-ensure
dep-ensure: dep-install ## Run dep ensure to generate vendor directory
	dep ensure

.PHONY: code-gen
code-gen: dep-ensure ## Generate boilerplate code for kubernetes packages
	@./hack/update-generated.sh

.PHONY: build-bin
build-bin: _output ## Build m3db-operator binary
	which go > /dev/null || (echo "error: golang needs to be installed" && exit 1)
	echo "building $(PROJECT_NAME)..."
	$(BUILD_SETTINGS) go build -o $(OUTPUT_DIR)/$(PROJECT_NAME) $(BUILD_PATH)

.PHONY: build-docker 
build-docker: build-bin## Build m3db-operator docker image with go binary
	@./build/build-docker.sh
