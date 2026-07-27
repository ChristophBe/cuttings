BINARY      := workstreams
BIN_DIR     := bin
INSTALL_DIR := $(shell go env GOPATH)/bin

.PHONY: build install test lint clean

## build: compile the binary to ./bin/workstreams
build:
	go build -o $(BIN_DIR)/$(BINARY) .

## install: install the binary to $GOPATH/bin
install:
	go install .

## test: run all tests
test:
	go test ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## clean: remove build artefacts
clean:
	rm -rf $(BIN_DIR)

## help: print this help
help:
	@sed -n 's/^## //p' $(MAKEFILE_LIST)
