.DEFAULT_GOAL := default

LANG=C
MAKEOVERRIDES =
targets:
	@printf "%-30s %s\n" "Target" "Description"
	@printf "%-30s %s\n" "------" "-----------"
	@make -pqR : 2>/dev/null \
	| awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' \
	| egrep -v -e '^[^[:alnum:]]' -e '^$@$$' \
	| sort \
	| xargs -I _ sh -c 'printf "%-30s " _; make _ -nB | (grep "^# Target:" || echo "") | tail -1 | sed "s/^# Target: //g"'

REPO    := github.com/pingcap/tiup

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAGS)
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

FAILPOINT_ENABLE  := $$(go tool github.com/pingcap/failpoint/failpoint-ctl enable)
FAILPOINT_DISABLE := $$(go tool github.com/pingcap/failpoint/failpoint-ctl disable)

default: check build
	@# Target: run the checks and then build.

include ../../tests/Makefile

# Build components
build: components
	@# Target: build tiup and all it's components

components: client
	@# Target: build the playground, client, cluster, dm and server components

client:
	@# Target: build the tiup-client component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o ../../bin/tiup-client .

check: fmt lint check-static vet
	@# Target: run all checkers. (fmt, lint, check-static and vet)

check-static:
	@# Target: run the golangci-lint static check tool
	../../tools/bin/golangci-lint run --config ../../tools/check/golangci.yaml ./... --timeout=3m --fix

lint:
	@# Target: run the lint checker revive
	@echo "linting"
	@go tool github.com/mgechev/revive -formatter friendly -config ../../tools/check/revive.toml $(FILES)

vet:
	@# Target: run the go vet tool
	$(GO) vet ./...

clean:
	@# Target: run the build cleanup steps
	@rm -rf bin
	@rm -rf tests/*/{bin/*.test,logs}

test: failpoint-enable run-tests failpoint-disable
	@# Target: run tests with failpoint enabled

# TODO: refactor integration tests base on v1 manifest
# run-tests: unit-test integration_test
run-tests: unit-test
	@# Target: run the unit tests

# Run tests
unit-test:
	@# Target: run the code coverage test phase
	mkdir -p ../../cover
	TIUP_HOME=$(shell pwd)/../../tests/tiup $(GOTEST) ./... -covermode=count -coverprofile ../../cover/cov.unit-test.client.out

race: failpoint-enable
	@# Target: run race check with failpoint enabled
	TIUP_HOME=$(shell pwd)/../../tests/tiup $(GOTEST) -race ./...  || { $(FAILPOINT_DISABLE); exit 1; }
	@$(FAILPOINT_DISABLE)

failpoint-enable:
	@# Target: enable failpoint
	@$(FAILPOINT_ENABLE)

failpoint-disable:
	@# Target: disable failpoint
	@$(FAILPOINT_DISABLE)

fmt:
	@# Target: run the go formatter utility
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1
	@echo "goimports (if installed)"
	$(shell goimports -w $(FILES) 2>/dev/null)

