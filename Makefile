BINARY      := workstreams
BIN_DIR     := bin
INSTALL_DIR := $(shell go env GOPATH)/bin

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X 'github.com/ChristophBe/workstreams/cmd.Version=$(VERSION)' \
              -X 'github.com/ChristophBe/workstreams/cmd.BuildTime=$(BUILD_TIME)'

.PHONY: build install test lint clean

## build: compile the binary to ./bin/workstreams
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

## install: install the binary to $GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" .

## test: run all tests
test:
	go test -race -coverprofile=coverage.out ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## clean: remove build artefacts
clean:
	rm -rf $(BIN_DIR)

## help: print this help
help:
	@sed -n 's/^## //p' $(MAKEFILE_LIST)
