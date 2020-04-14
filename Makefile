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
BUILDTIME := $(shell date '+%Y-%m-%d %T %z')

REPO := github.com/pingcap-incubator/tiup-cluster
LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitBranch=$(BRANCH)"
LDFLAGS += $(EXTRA_LDFLAGS)

FILES     := $$(find . -name "*.go")

FAILPOINT_ENABLE  := $$(find $$PWD/ -type d | grep -vE "(\.git|tools)" | xargs tools/bin/failpoint-ctl enable)
FAILPOINT_DISABLE := $$(find $$PWD/ -type d | grep -vE "(\.git|tools)" | xargs tools/bin/failpoint-ctl disable)

default: check build

build:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-cluster

# lint:
# 	@golint ./...
lint:tools/bin/revive
	@echo "linting"
	@tools/bin/revive -formatter friendly -config tools/check/revive.toml $(FILES)

vet:
	$(GO) vet ./...

check: lint vet fmt check-static

clean:
	@rm -rf bin

cover-dir:
	rm -rf cover
	mkdir -p cover

# Run tests
unit-test: cover-dir
	$(GOTEST) ./... -covermode=count -coverprofile cover/cov.unit-test.out

integration_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap-incubator/tiup-cluster/... \
		-o tests/bin/tiup-cluster \
		github.com/pingcap-incubator/tiup-cluster/ ; \
	cd tests && sh run.sh ; \


test: failpoint-enable unit-test #integration_test
	@$(FAILPOINT_DISABLE)

check-static: tools/bin/golangci-lint
	tools/bin/golangci-lint run --timeout 5m ./...

coverage:
	GO111MODULE=off go get github.com/wadey/gocovmerge
	gocovmerge cover/cov.* | grep -vE ".*.pb.go|.*__failpoint_binding__.go" > "cover/all_cov.out"
ifeq ("$(JenkinsCI)", "1")
	@bash <(curl -s https://codecov.io/bash) -f cover/all_cov.out -t $(CODECOV_TOKEN)
endif

failpoint-enable: tools/bin/failpoint-ctl
	@$(FAILPOINT_ENABLE)

failpoint-disable: tools/bin/failpoint-ctl
	@$(FAILPOINT_DISABLE)

tools/bin/failpoint-ctl: go.mod
	$(GO) build -o $@ github.com/pingcap/failpoint/failpoint-ctl

tools/bin/revive: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/revive github.com/mgechev/revive

tools/bin/golangci-lint: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

fmt:
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1

.PHONY: build package
