GOVER := $(shell go version)

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)
GOTEST := $(GO) test

COMMIT    := $(shell git describe --no-match --always --dirty)
BRANCH    := $(shell git rev-parse --abbrev-ref HEAD)
BUILDTIME := $(shell date '+%Y-%m-%d %T %z')

REPO := github.com/pingcap-incubator/tiup
LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitBranch=$(BRANCH)"
LDFLAGS += -X "$(REPO)/pkg/version.BuildTime=$(BUILDTIME)"
LDFLAGS += $(EXTRA_LDFLAGS)

FILES     := $$(find . -name "*.go")

FAILPOINT_ENABLE  := $$(find $$PWD/ -type d | grep -vE "(\.git|tools)" | xargs tools/bin/failpoint-ctl enable)
FAILPOINT_DISABLE := $$(find $$PWD/ -type d | grep -vE "(\.git|tools)" | xargs tools/bin/failpoint-ctl disable)

default: check cmd

cmd:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup

lint:
	@golint ./...

vet:
	$(GO) vet ./...

check: lint vet

clean:
	@rm -rf bin

# Run tests
test: failpoint-enable
	rm -rf cover.* cover
	mkdir -p cover
	$(GOTEST) ./... -coverprofile cover.out.tmp
	cat cover.out.tmp | grep -v "_generated.deepcopy.go" > cover.out
	@$(FAILPOINT_DISABLE)

coverage:
ifeq ("$(JenkinsCI)", "1")
	@bash <(curl -s https://codecov.io/bash) -f cover.out -t $(CODECOV_TOKEN)
endif

failpoint-enable: tools/bin/failpoint-ctl
	@$(FAILPOINT_ENABLE)

failpoint-disable: tools/bin/failpoint-ctl
	@$(FAILPOINT_DISABLE)

tools/bin/failpoint-ctl: go.mod
	$(GO) build -o $@ github.com/pingcap/failpoint/failpoint-ctl

playground:
	make -C components/playground package

client:
	make -C components/client package

pack:
	make -C components/package package

package: playground client pack
	mkdir -p package ; \
	GOOS=darwin GOARCH=amd64 go build ; \
    tar -czf tiup-darwin-amd64.tar.gz tiup ; \
    shasum tiup-darwin-amd64.tar.gz | awk '{print $$1}' > tiup-darwin-amd64.sha1 ; \
    GOOS=linux GOARCH=amd64 go build ; \
    tar -czf tiup-linux-amd64.tar.gz tiup ; \
    shasum tiup-linux-amd64.tar.gz | awk '{print $$1}' > tiup-linux-amd64.sha1 ; \
    rm tiup ; \
    mv tiup* package/ ; \
    mv components/playground/playground* package/ ; \
	mv components/client/client* package/ ; \
	mv components/package/package* package/ ; \
    cp mirror/*.index package/
	cp install.sh package/

fmt:
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1

.PHONY: cmd package
