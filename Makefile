# Go lives only inside Docker; nothing is installed on the host.
# Module and build caches live in the Docker volume $(CACHE_VOLUME), not in the project dir.
# `make clean` removes build output; `make clean-cache` also removes the volumes and the app image.

GO_IMAGE     ?= golang:1.27.2
NODE_IMAGE   ?= node:24.21-bookworm-slim
APP_IMAGE    ?= quipu-app-build
CACHE_VOLUME ?= quipu-go-cache
NODE_VOLUMES  = quipu-node-cache quipu-node-modules
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

# Node only builds the frontend (web/ → internal/ui/dist/app). node_modules and the npm cache live
# in Docker volumes, so web/ holds only sources.
DOCKER_NODE = docker run --rm \
	-u $(shell id -u):$(shell id -g) \
	-v $(CURDIR)/web:/web -w /web \
	-v $(CURDIR)/internal/ui/dist:/internal/ui/dist \
	-v quipu-node-modules:/web/node_modules \
	-v quipu-node-cache:/npm \
	-e HOME=/tmp -e npm_config_cache=/npm

.PHONY: build app app-image web web-dev test vet fmt tidy shell clean clean-cache cache-volume node-volumes

build: cache-volume web app
	$(DOCKER_GO) go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/quipu ./cmd/quipu

# The native window (ADR 0007): links GTK 3 and WebKitGTK, so it builds in its own image.
app: cache-volume app-image
	$(DOCKER_RUN) -t $(APP_IMAGE) go build -trimpath -tags desktop,production,webkit2_41 \
		-ldflags "-s -w -X main.version=$(VERSION)" -o bin/quipu-app ./cmd/quipu-app

app-image:
	@docker image inspect $(APP_IMAGE) >/dev/null 2>&1 || \
		docker build -t $(APP_IMAGE) --build-arg GO_IMAGE=$(GO_IMAGE) -f build/app.Dockerfile build

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

web: node-volumes
	$(DOCKER_NODE) -t $(NODE_IMAGE) sh -c "npm ci --no-audit --no-fund && npm run build"

# Vite dev server on http://127.0.0.1:5173 with hot reload; run `quipu ui --port 47801` next to it.
web-dev: node-volumes
	$(DOCKER_NODE) -it --network host $(NODE_IMAGE) sh -c "npm ci --no-audit --no-fund && npm run dev"

node-volumes:
	@for v in $(NODE_VOLUMES); do docker volume inspect $$v >/dev/null 2>&1 || { \
		docker volume create $$v >/dev/null && \
		docker run --rm -v $$v:/v $(NODE_IMAGE) chown $(shell id -u):$(shell id -g) /v; }; done

# A new named volume is owned by root; hand it to the current user once.
cache-volume:
	@docker volume inspect $(CACHE_VOLUME) >/dev/null 2>&1 || { \
		docker volume create $(CACHE_VOLUME) >/dev/null && \
		docker run --rm -v $(CACHE_VOLUME):/cache $(GO_IMAGE) chown $(shell id -u):$(shell id -g) /cache; }

clean:
	rm -rf bin internal/ui/dist/app

clean-cache: clean
	-docker volume rm $(CACHE_VOLUME) $(NODE_VOLUMES)
	-docker image rm $(APP_IMAGE)
