# Go lives only inside Docker; nothing is installed on the host.
# Module and build caches are kept in ./.cache (gitignored, `make clean` removes it).

GO_IMAGE ?= golang:1.27.2
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

DOCKER_GO = docker run --rm -t \
	-u $(shell id -u):$(shell id -g) \
	-v $(CURDIR):/src -w /src \
	-e HOME=/tmp \
	-e GOMODCACHE=/src/.cache/mod \
	-e GOCACHE=/src/.cache/build \
	-e CGO_ENABLED=1 \
	-e GOOS -e GOARCH \
	$(GO_IMAGE)

.PHONY: build test vet fmt tidy shell clean

build:
	$(DOCKER_GO) go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/quipu ./cmd/quipu

test:
	$(DOCKER_GO) go test ./...

vet:
	$(DOCKER_GO) go vet ./...

fmt:
	$(DOCKER_GO) go fmt ./...

tidy:
	$(DOCKER_GO) go mod tidy

shell:
	docker run --rm -it -u $(shell id -u):$(shell id -g) -v $(CURDIR):/src -w /src \
		-e HOME=/tmp -e GOMODCACHE=/src/.cache/mod -e GOCACHE=/src/.cache/build $(GO_IMAGE) bash

clean:
	rm -rf bin .cache
