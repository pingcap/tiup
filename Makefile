.PHONY: components server targets
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

FAILPOINT_ENABLE  := $$(tools/bin/failpoint-ctl enable)
FAILPOINT_DISABLE := $$(tools/bin/failpoint-ctl disable)

default: check build
	@# Target: run the checks and then build.

include ./tests/Makefile

# Build TiUP and all components
build: tiup components
	@# Target: build tiup and all it's components

components: playground client cluster dm bench server
	@# Target: build the playground, client, cluster, dm, bench and server components

tiup:
	@# Target: build the tiup driver
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup

playground:
	@# Target: build tiup-playground component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-playground ./components/playground

client:
	@# Target: build the tiup-client component
	$(MAKE) -C components/client $(MAKECMDGOALS)

cluster:
	@# Target: build the tiup-cluster component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-cluster ./components/cluster

dm:
	@# Target: build the tiup-dm component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-dm ./components/dm

bench:
	@# Target: build the tiup-bench component
	$(MAKE) -C components/bench $(MAKECMDGOALS)

doc:
	@# Target: build the tiup-doc component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-doc ./components/doc

errdoc:
	@# Target: build the tiup-errdoc component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-errdoc ./components/errdoc

ctl:
	@# Target: build the tiup-ctl component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-ctl ./components/ctl

server:
	@# Target: build the tiup-server component
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-server ./server

check: fmt lint tidy check-static vet
	@# Target: run all checkers. (fmt, lint, tidy, check-static and vet)
	$(MAKE) -C components/bench ${MAKECMDGOALS}
	$(MAKE) -C components/client ${MAKECMDGOALS}

check-static: tools/bin/golangci-lint
	@# Target: run the golangci-lint static check tool
	tools/bin/golangci-lint run --config tools/check/golangci.yaml ./... --deadline=3m --fix

lint: tools/bin/revive
	@# Target: run the lint checker revive
	@echo "linting"
	./tools/check/check-lint.sh
	@tools/bin/revive -formatter friendly -config tools/check/revive.toml $(FILES)

vet:
	@# Target: run the go vet tool
	$(GO) vet ./...

tidy:
	@# Target: run tidy check
	@echo "go mod tidy"
	./tools/check/check-tidy.sh

clean:
	@# Target: run the build cleanup steps
	@rm -rf bin
	@rm -rf cover
	@rm -rf tests/*/{bin/*.test,logs,cover/*.out}

test: failpoint-enable run-tests failpoint-disable
	@# Target: run tests with failpoint enabled
	$(MAKE) -C components/bench ${MAKECMDGOALS}
	$(MAKE) -C components/client ${MAKECMDGOALS}

# TODO: refactor integration tests base on v1 manifest
# run-tests: unit-test integration_test
run-tests: unit-test
	@# Target: run the unit tests

# Run tests
unit-test:
	@# Target: run the code coverage test phase
	mkdir -p cover
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) ./... -covermode=count -coverprofile cover/cov.unit-test.out

race: failpoint-enable
	@# Target: run race check with failpoint enabled
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) -race ./...  || { $(FAILPOINT_DISABLE); exit 1; }
	@$(FAILPOINT_DISABLE)

failpoint-enable: tools/bin/failpoint-ctl
	@# Target: enable failpoint
	@$(FAILPOINT_ENABLE)

failpoint-disable: tools/bin/failpoint-ctl
	@# Target: disable failpoint
	@$(FAILPOINT_DISABLE)

tools/bin/failpoint-ctl: go.mod
	@# Target: build the failpoint-ctl utility
	$(GO) build -o $@ github.com/pingcap/failpoint/failpoint-ctl

fmt:
	@# Target: run the go formatter utility
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1
	@echo "goimports (if installed)"
	$(shell goimports -w $(FILES) 2>/dev/null)

tools/bin/revive: tools/check/go.mod
	@# Target: build revive utility
	cd tools/check; \
	$(GO) build -o ../bin/revive github.com/mgechev/revive

tools/bin/golangci-lint:
	@# Target: pull in specific version of golangci-lint (v1.42.1)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./tools/bin v1.45.0
