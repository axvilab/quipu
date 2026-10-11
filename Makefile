# Go lives only inside Docker; nothing is installed on the host.
# Module and build caches live in the Docker volume $(CACHE_VOLUME), not in the project dir.
# `make clean` removes bin/; `make clean-cache` also removes the volume.

GO_IMAGE     ?= golang:1.27.2
CACHE_VOLUME ?= quipu-go-cache
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

DOCKER_RUN = docker run --rm \
	-u $(shell id -u):$(shell id -g) \
	-v $(CURDIR):/src -w /src \
	-v $(CACHE_VOLUME):/cache \
	-e HOME=/tmp \
	-e GOMODCACHE=/cache/mod \
	-e GOCACHE=/cache/build \
	-e CGO_ENABLED=1 \
	-e GOOS -e GOARCH

DOCKER_GO = $(DOCKER_RUN) -t $(GO_IMAGE)

.PHONY: build test vet fmt tidy shell clean clean-cache cache-volume

build: cache-volume
	$(DOCKER_GO) go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/quipu ./cmd/quipu

test: cache-volume
	$(DOCKER_GO) go test ./...

vet: cache-volume
	$(DOCKER_GO) go vet ./...

fmt: cache-volume
	$(DOCKER_GO) go fmt ./...

tidy: cache-volume
	$(DOCKER_GO) go mod tidy

shell: cache-volume
	$(DOCKER_RUN) -it $(GO_IMAGE) bash

# A new named volume is owned by root; hand it to the current user once.
cache-volume:
	@docker volume inspect $(CACHE_VOLUME) >/dev/null 2>&1 || { \
		docker volume create $(CACHE_VOLUME) >/dev/null && \
		docker run --rm -v $(CACHE_VOLUME):/cache $(GO_IMAGE) chown $(shell id -u):$(shell id -g) /cache; }

clean:
	rm -rf bin

clean-cache: clean
	-docker volume rm $(CACHE_VOLUME)
