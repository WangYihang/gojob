GO ?= go
PKGS := ./...
# Library packages to measure for coverage (runnable examples excluded).
COVERPKGS = $$($(GO) list ./... | grep -v /examples/)

.PHONY: all build vet fmt test test-integration test-e2e test-all cover cover-html clean

## all: format, vet, and run unit tests
all: fmt vet test

## build: compile every package and example
build:
	$(GO) build $(PKGS)

## vet: run go vet
vet:
	$(GO) vet $(PKGS)

## fmt: format all Go files
fmt:
	$(GO) fmt $(PKGS)

## test: unit tests with the race detector
test:
	$(GO) test -race $(PKGS)

## test-integration: unit + integration tests (local HTTP servers)
test-integration:
	$(GO) test -race -tags=integration $(PKGS)

## test-e2e: build example binaries and drive them end-to-end
test-e2e:
	$(GO) test -tags=e2e $(PKGS)

## test-all: unit + integration + e2e
test-all:
	$(GO) test -race -tags='integration e2e' $(PKGS)

## cover: coverage profile over unit + integration tests (library packages only)
cover:
	$(GO) test -tags=integration -coverprofile=coverage.txt -covermode=atomic $(COVERPKGS)
	@$(GO) tool cover -func=coverage.txt | tail -1

## cover-html: render the coverage profile to coverage.html
cover-html: cover
	$(GO) tool cover -html=coverage.txt -o coverage.html

## clean: remove build and coverage artifacts
clean:
	rm -f coverage.txt coverage.html
	$(GO) clean
