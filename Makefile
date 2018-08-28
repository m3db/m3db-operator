PROJECT_NAME := m3db-operator
REPO_PATH    := github.com/m3db/m3db-operator
BUILD_PATH   := $(REPO_PATH)/cmd/$(PROJECT_NAME)
OUTPUT_DIR   := _output
BUILD_SETTINGS := GOOS=linux GOARCH=amd64 CGO_ENABLED=0 
ifeq ($(shell uname), Darwin)
	BUILD_SETTINGS := GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 
endif
ifeq ($(LINUX_BUILD), 1)
	BUILD_SETTINGS := GOOS=linux GOARCH=amd64 CGO_ENABLED=0 
endif

.PHONY: all
all: clean code-gen build-docker 

.PHONY: help
help:
	@echo
	@echo "\033[92m  Usage:"
	@printf "    \033[36m%-15s\033[93m %s" "make" "<target>"
	@echo
	@echo
	@echo "\033[92m  Targets: "
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-15s\033[93m %s\n", $$1, $$2}'
	@printf "\033[0m"

.PHONY: clean 
clean:
	go clean
	rm -f build/output/bin/m3db-operator

_output:
	mkdir -p $$(pwd)/$(OUTPUT_DIR)

.PHONY: test
test: $(wildcard **/*.go) _output
	go test -coverprofile=.tmp/c.out ./...
	go tool cover -func=.tmp/c.out

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
