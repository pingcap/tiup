.PHONY: cmd components server
.DEFAULT_GOAL := default

REPO    := github.com/pingcap/tiup

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)
GOTEST  := GO111MODULE=on CGO_ENABLED=1 go test -p 3
SHELL   := /usr/bin/env bash

_COMMIT := $(shell git describe --no-match --always --dirty)
_GITREF := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT  := $(if $(COMMIT),$(COMMIT),$(_COMMIT))
GITREF  := $(if $(GITREF),$(GITREF),$(_GITREF))

LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitRef=$(GITREF)"
LDFLAGS += $(EXTRA_LDFLAGS)

FILES   := $$(find . -name "*.go")

FAILPOINT_ENABLE  := $$(tools/bin/failpoint-ctl enable)
FAILPOINT_DISABLE := $$(tools/bin/failpoint-ctl disable)

default: check build

include ./tests/Makefile

# Build TiUP and all components
build: tiup components

components: playground client cluster dm bench server

tiup:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup

playground:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-playground ./components/playground

client:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-client ./components/client

cluster:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-cluster ./components/cluster

dm:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-dm ./components/dm

bench:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-bench ./components/bench

doc:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-doc ./components/doc

errdoc:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-errdoc ./components/errdoc

server:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-server ./server

check: fmt lint tidy check-static vet

check-static: tools/bin/golangci-lint
	tools/bin/golangci-lint run --config tools/check/golangci.yaml ./... --deadline=3m --fix

lint: tools/bin/revive
	@echo "linting"
	./tools/check/check-lint.sh
	@tools/bin/revive -formatter friendly -config tools/check/revive.toml $(FILES)

vet:
	$(GO) vet ./...

tidy:
	@echo "go mod tidy"
	./tools/check/check-tidy.sh

clean:
	@rm -rf bin
	@rm -rf cover

test: failpoint-enable run-tests failpoint-disable

# TODO: refactor integration tests base on v1 manifest
# run-tests: unit-test integration_test
run-tests: unit-test

# Run tests
unit-test:
	mkdir -p cover
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) ./... -covermode=count -coverprofile cover/cov.unit-test.out

race: failpoint-enable
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) -race ./...  || { $(FAILPOINT_DISABLE); exit 1; }
	@$(FAILPOINT_DISABLE)

failpoint-enable: tools/bin/failpoint-ctl
	@$(FAILPOINT_ENABLE)

failpoint-disable: tools/bin/failpoint-ctl
	@$(FAILPOINT_DISABLE)

tools/bin/failpoint-ctl: go.mod
	$(GO) build -o $@ github.com/pingcap/failpoint/failpoint-ctl

fmt:
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1
	@echo "goimports (if installed)"
	$(shell goimports -w $(FILES) 2>/dev/null)

tools/bin/revive: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/revive github.com/mgechev/revive

tools/bin/golangci-lint:
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ./tools/bin v1.27.0
