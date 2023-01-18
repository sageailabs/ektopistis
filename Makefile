DOCKER_IMAGE ?= ektopistis
DOCKER_IMAGE_TAG ?= latest

.PHONY: docker/build
docker/build:
	docker build --pull --tag=$(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) .

.PHONY: build
build: ektopistis

ektopistis: go.mod main.go node-drainer.go
	go mod tidy
	go build -o ektopistis

.PHONY: lint
lint:
	go vet ./...

.PHONY: test
test:
	go test -v ./...
