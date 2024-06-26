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
	CGO_ENABLED=0 go build -o ${BINARY_NAME}

.PHONY: lint
lint: ## Lint using gofmt and govet
	go vet ./...
	go fmt ./...

.PHONY: test
test: ## Run all tests
	go run github.com/onsi/ginkgo/v2/ginkgo -r

.PHONY: test/coverage
test/coverage: ## Run all tests under coverage and generate coverage report
	go run github.com/onsi/ginkgo/v2/ginkgo -r --coverprofile=coverage.txt
	go tool cover --html=coverage.txt -o coverage.html

.PHONY: clean
clean: ## Clean build artifacts
	go clean
	rm ${BINARY_NAME} coverage.txt coverage.html

##@ Docker

.PHONY: docker/build
docker/build: ## Build the ektopistis docker image
	docker build --pull --tag=$(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) .
