.PHONY: all
all: clean build-bin build-docker deploy

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

.PHONY: code-gen
code-gen: ## Generate boilerplate code for kubernetes packages
	@./codegen/update-generated.sh

.PHONY: build-bin
build-bin: code-gen ## Build m3db-operator binary
	@./build/build.sh

.PHONY: build-docker 
build-docker: build-bin ## Build m3db-operator docker image with go binary
	@./build/docker_build.sh

.PHONY: deploy
deploy: build-docker ## Deploy artifacts using current git sha 
	@./deploy/deployment.sh
