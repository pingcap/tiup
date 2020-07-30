.PHONY: cmd components server
.DEFAULT_GOAL := default

GOVER := $(shell go version)

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)
GOTEST  := GO111MODULE=on CGO_ENABLED=1 $(GO) test -p 3
SHELL   := /usr/bin/env bash

COMMIT    := $(shell git describe --no-match --always --dirty)
BRANCH    := $(shell git rev-parse --abbrev-ref HEAD)

REPO := github.com/pingcap/tiup
LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitBranch=$(BRANCH)"
LDFLAGS += $(EXTRA_LDFLAGS)

FILES     := $$(find . -name "*.go")

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

err:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-err ./components/err

server:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-server ./server

check: fmt lint tidy check-static vet

check-static: tools/bin/golangci-lint
	tools/bin/golangci-lint run ./... --deadline=3m

lint:tools/bin/revive
	@echo "linting"
	@tools/bin/revive -formatter friendly -config tools/check/revive.toml $(FILES)

vet:
	$(GO) vet ./...

tidy:
	@echo "go mod tidy"
	./tools/check/check-tidy.sh

clean:
	@rm -rf bin

cover-dir:
	rm -rf cover
	mkdir -p cover

# Run tests
unit-test: cover-dir
	TIUP_HOME=$(shell pwd)/tests/tiup $(GOTEST) ./... -covermode=count -coverprofile cover/cov.unit-test.out

test: cover-dir failpoint-enable
	make run-tests; STATUS=$$?; $(FAILPOINT_DISABLE); exit $$STATUS

# TODO: refactor integration tests base on v1 manifest
# run-tests: unit-test integration_test
run-tests: unit-test

coverage:
	GO111MODULE=off go get github.com/wadey/gocovmerge
	gocovmerge cover/cov.* | grep -vE ".*.pb.go|.*__failpoint_binding__.go|mock.go" > "cover/all_cov.out"
ifeq ("$(JenkinsCI)", "1")
	@bash <(curl -s https://codecov.io/bash) -f cover/all_cov.out -t $(CODECOV_TOKEN)
else
	go tool cover -html "cover/all_cov.out" -o "cover/all_cov.html"
endif

failpoint-enable: tools/bin/failpoint-ctl
	@$(FAILPOINT_ENABLE)

failpoint-disable: tools/bin/failpoint-ctl
	@$(FAILPOINT_DISABLE)

tools/bin/failpoint-ctl: go.mod
	$(GO) build -o $@ github.com/pingcap/failpoint/failpoint-ctl

pkger:
	 $(GO) run tools/pkger/main.go -s templates -d pkg/cluster/embed

fmt:
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1
	@echo "goimports (if installed)"
	$(shell gimports -w $(FILES) 2>/dev/null)

tools/bin/errcheck: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/errcheck github.com/kisielk/errcheck

tools/bin/revive: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/revive github.com/mgechev/revive

tools/bin/golangci-lint:
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ./tools/bin v1.27.0

