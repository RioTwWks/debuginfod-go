# Переменные
BINARY_NAME=debuginfod
GO=go
GOPATH?=$(shell go env GOPATH)
SQLITE_DB=debuginfod.sqlite

.PHONY: all build test vet run run-env clean lint fmt docker

all: build

build:
	$(GO) build -o $(BINARY_NAME) ./cmd/debuginfod

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
	rm -f $(BINARY_NAME) $(SQLITE_DB)
	rm -rf .debuginfod-cache
	$(GO) clean

docker: build
	docker build -t debuginfod-go .
