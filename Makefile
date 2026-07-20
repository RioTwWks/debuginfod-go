# Переменные
BINARY_NAME=debuginfod
GO=go
GOPATH?=$(shell go env GOPATH)
SQLITE_DB=debuginfod.sqlite

.PHONY: all build build-find build-bench-dedup bench-dedup test vet run run-env clean lint fmt docker \
	docker-prep docker-prebuilt docker-up-prebuilt docker-astra docker-up-astra \
	package package-deb package-rpm \
	offline-download-deb offline-download-rpm \
	offline-bundle-deb offline-bundle-rpm

all: build

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo 0.1.0)
NFPM ?= nfpm
DIST_DIR = dist
OFFLINE_DIR = dist/offline

build:
	$(GO) build -o $(BINARY_NAME) ./cmd/debuginfod

build-find:
	$(GO) build -o debuginfod-find ./cmd/debuginfod-find

build-bench-dedup:
	$(GO) build -o bench-dedup ./cmd/bench-dedup

bench-dedup: build-bench-dedup

test:
	$(GO) test -v ./...

vet:
	$(GO) vet ./...

run: build
	./$(BINARY_NAME) -s /usr/lib/debug -p 8002

run-env: build
	./$(BINARY_NAME)

lint:
	golangci-lint run ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -f $(BINARY_NAME) debuginfod-find $(SQLITE_DB)
	rm -rf .debuginfod-cache $(DIST_DIR)
	$(GO) clean

# --- Пакеты .deb / .rpm (nfpm) ---

dist:
	mkdir -p $(DIST_DIR)

package-bin: dist build build-find
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags "-s -w" -o $(DIST_DIR)/debuginfod ./cmd/debuginfod
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags "-s -w" -o $(DIST_DIR)/debuginfod-find ./cmd/debuginfod-find

package-deb: package-bin
	VERSION=$(VERSION) $(NFPM) package -f deploy/nfpm.yaml -p deb -t $(DIST_DIR) --packager deb

package-rpm: package-bin
	VERSION=$(VERSION) $(NFPM) package -f deploy/nfpm.yaml -p rpm -t $(DIST_DIR) --packager rpm

package: package-deb package-rpm

# --- Оффлайн bundle (скачивание на online-хосте) ---

offline-download-deb:
	bash deploy/offline/download-deps-deb.sh

offline-download-rpm:
	bash deploy/offline/download-deps-rpm.sh

offline-bundle-deb:
	bash deploy/offline/download-deps-deb.sh
	VERSION=$(VERSION) bash deploy/offline/make-bundle.sh deb

offline-bundle-rpm:
	bash deploy/offline/download-deps-rpm.sh
	VERSION=$(VERSION) bash deploy/offline/make-bundle.sh rpm

# --- Docker (dev/demo) ---

.PHONY: docker-prep
docker-prep:
	bash deploy/docker/prepare-build-certs.sh
	bash deploy/docker/ensure-proxy-env.sh

docker-prebuilt: build docker-prep
	docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml up --build

docker-up-prebuilt: build docker-prep
	docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml up -d --build

docker-astra: build docker-prep
	docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml -f docker-compose.astra.yml up --build

docker-up-astra: build docker-prep
	docker compose -f docker-compose.yml -f docker-compose.prebuilt.yml -f docker-compose.astra.yml up -d --build

docker: build docker-prep
	docker build -t debuginfod-go .
