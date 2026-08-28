BINARY      := workstreams
BIN_DIR     := bin
INSTALL_DIR := $(shell go env GOPATH)/bin

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
              -X 'github.com/ChristophBe/workstreams/cmd.Version=$(VERSION)' \
              -X 'github.com/ChristophBe/workstreams/cmd.BuildTime=$(BUILD_TIME)'

.PHONY: build install test e2e lint clean tag generate-docs site site-dev help

## build: compile the binary to ./bin/workstreams
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

## install: install the binary to $GOPATH/bin
install:
	go install -trimpath -ldflags "$(LDFLAGS)" .

## test: run all unit tests
test:
	go test -race -coverprofile=coverage.out ./...

## e2e: run end-to-end CLI tests (builds the binary and exercises it as a black box)
e2e:
	go test -tags=e2e -count=1 ./e2e/...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## clean: remove build artefacts
clean:
	rm -rf $(BIN_DIR)

## generate-docs: regenerate the command reference in docs/features.md
generate-docs:
	go run ./tools/gendocs

## site: build the GitHub Pages site to site/public
site:
	cd site && hugo --minify

## site-dev: serve the GitHub Pages site locally with live reload
site-dev:
	cd site && hugo server

## tag: create and push a release tag (usage: make tag VERSION=v1.2.3)
tag:
	@[ -n "$(VERSION)" ] || (echo "Usage: make tag VERSION=v1.2.3" && exit 1)
	git tag $(VERSION)
	git push origin $(VERSION)

## help: print this help
help:
	@sed -n 's/^## //p' $(MAKEFILE_LIST)
