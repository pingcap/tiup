.PHONY: server

GOVER := $(shell go version)

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)
GOTEST  := GO111MODULE=on CGO_ENABLED=1 $(GO) test -p 3
SHELL   := /usr/bin/env bash

REPO := github.com/pingcap/tiup
LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitBranch=$(BRANCH)"
LDFLAGS += $(EXTRA_LDFLAGS)

FILES     := $$(find . -name "*.go")

build_integration_test: build_tiup_playground_test build_tiup_cluster_test build_tiup_dms_test

build_tiup_playground_test: failpoint-enable
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-playground/bin/tiup-playground.test \
		github.com/pingcap/tiup/components/playground

build_tiup_cluster_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-cluster/bin/tiup-cluster.test \
		github.com/pingcap/tiup/components/cluster;

build_tiup_dms_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup-dms/bin/tiup-dms.test \
		github.com/pingcap/tiup/components/dms;

build_tiup_test:
	$(GOTEST) -c -cover -covermode=count \
		-coverpkg=github.com/pingcap/tiup/... \
		-o tests/tiup/bin/tiup.test \
		github.com/pingcap/tiup;