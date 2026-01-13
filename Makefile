.PHONY: targets
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
GOINSTALL := $(GO) install
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

include ./tests/Makefile

# Build TiUP and all components
.PHONY: build
build: tiup components
	@# Target: build tiup and all it's components

.PHONY: components
components: playground client cluster dm server
	@# Target: build the playground, client, cluster, dm and server components

.PHONY: tiup
tiup:
	@# Target: build the tiup driver
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup

.PHONY: playground
playground:
	@# Target: build tiup-playground component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-playground ./components/playground

.PHONY: client
client:
	@# Target: build the tiup-client component
	$(MAKE) -C components/client $(MAKECMDGOALS)

.PHONY: cluster
cluster:
	@# Target: build the tiup-cluster component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-cluster ./components/cluster

.PHONY: dm
dm:
	@# Target: build the tiup-dm component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-dm ./components/dm

.PHONY: ctl
ctl:
	@# Target: build the tiup-ctl component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-ctl ./components/ctl

.PHONY: server
server:
	@# Target: build the tiup-server component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-server ./server

.PHONY: check
check: fmt lint tidy check-static vet
	@# Target: run all checkers. (fmt, lint, tidy, check-static and vet)
	$(MAKE) -C components/client ${MAKECMDGOALS}

.PHONY: check-static
check-static: tools/bin/golangci-lint
	@# Target: run the golangci-lint static check tool
	tools/bin/golangci-lint run --config tools/check/golangci.yaml ./... --timeout=3m --fix

.PHONY: lint
lint:
	@# Target: run the lint checker revive
	@echo "linting"
	@go tool github.com/mgechev/revive -formatter friendly -config tools/check/revive.toml $(FILES)

.PHONY: vet
vet:
	@# Target: run the go vet tool
	$(GO) vet ./...

.PHONY: tidy
tidy:
	@# Target: run tidy check
	@echo "go mod tidy"
	./tools/check/check-tidy.sh

.PHONY: clean
clean:
	@# Target: run the build cleanup steps
	@rm -rf bin
	@rm -rf cover
	@rm -rf tests/*/{bin/*.test,logs,cover/*.out}

.PHONY: test
test: failpoint-enable run-tests failpoint-disable
	@# Target: run tests with failpoint enabled
	$(MAKE) -C components/client ${MAKECMDGOALS}

# TODO: refactor integration tests base on v1 manifest
# run-tests: unit-test integration_test
.PHONY: run-tests
run-tests: unit-test
	@# Target: run the unit tests

# Run tests
.PHONY: unit-test
unit-test:
	@# Target: run the code coverage test phase
	mkdir -p cover
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) ./... -covermode=count -coverprofile cover/cov.unit-test.out

.PHONY: race
race: failpoint-enable
	@# Target: run race check with failpoint enabled
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) -race ./...  || { $(FAILPOINT_DISABLE); exit 1; }
	@$(FAILPOINT_DISABLE)

.PHONY: failpoint-enable
failpoint-enable:
	@# Target: enable failpoint
	@$(FAILPOINT_ENABLE)

.PHONY: failpoint-disable
failpoint-disable:
	@# Target: disable failpoint
	@$(FAILPOINT_DISABLE)

.PHONY: fmt
fmt:
	@# Target: run the go formatter utility
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1
	@echo "goimports (if installed)"
	$(shell goimports -w $(FILES) 2>/dev/null)

tools/bin/golangci-lint:
	@# Target: pull in specific version of golangci-lint (v2.7.2)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./tools/bin v2.7.2
