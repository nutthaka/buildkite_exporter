export GOOS := linux
export GOARCH := amd64

NAME := buildkite_exporter
VERSION := 0.1.1

pkgs = $(shell go list ./... | grep -v /vendor/)

all: style vet build test

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

test:
	@echo ">> running tests"
	@go test -short $(pkgs)

vet:
	@echo ">> vetting code"
	@go vet $(pkgs)

build:
	@echo ">> building binaries"
	@go build

tarball: build
	@echo ">> creating tarball"
	@tar -czf $(NAME)_$(VERSION)_$(GOOS)_$(GOARCH).tar.gz buildkite_exporter


.PHONY: all style build test vet tarball
