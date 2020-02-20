GOVER := $(shell go version)

GOOS    := $(if $(GOOS),$(GOOS),linux)
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)

COMMIT    := $(shell git describe --no-match --always --dirty)
BRANCH    := $(shell git rev-parse --abbrev-ref HEAD)
BUILDTIME := $(shell date '+%Y-%m-%d %T %z')

REPO := github.com/c4pt0r/tiup-demo/tiup
LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitBranch=$(BRANCH)"
LDFLAGS += -X "$(REPO)/pkg/version.BuildTime=$(BUILDTIME)"

default: cmd

cmd: check
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup-$(GOOS)-$(GOARCH) cmd/tiup.go

lint:
	@golint ./...

vet:
	$(GO) vet ./...

check: lint vet

clean:
	@rm -rf bin

.PHONY: cmd
