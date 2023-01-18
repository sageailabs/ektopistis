BINARY_NAME ?= ektopistis
DOCKER_IMAGE ?= ektopistis
DOCKER_IMAGE_TAG ?= latest

.DEFAULT_GOAL:=help

.PHONY: help
help:
	@awk ' \
		BEGIN {FS = ":.*##"; printf "Usage:\n  make [target] \033[36m\033[0m\n"} \
		/^[a-zA-Z_\-\/]+:.*?##/ { printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } \
	' $(MAKEFILE_LIST)

##@ Common

.PHONY: build
build: go.mod main.go node-drainer.go ## Build the ektopistis binary
	go mod tidy
	go build -o ${BINARY_NAME}

.PHONY: lint
lint: ## Lint using gofmt and govet
	go fmt ./...
	go vet ./...

.PHONY: test
test: ## Run all tests
	go test -v ./...

.PHONY: clean
clean: ## Clean build artifacts
	go clean
	rm ${BINARY_NAME}

##@ Docker

.PHONY: docker/build
docker/build: ## Build the ektopistis docker image
	docker build --pull --tag=$(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) .
