GOVER := $(shell go version)

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)
VERSION := "v0.2.0"

default: build

build: check
	$(GO) build -o bin/tiops

lint:
	@golint ./...

vet:
	$(GO) vet ./...

check: lint vet

clean:
	@rm -rf bin

failpoint-enable: tools/bin/failpoint-ctl
	@$(FAILPOINT_ENABLE)

failpoint-disable: tools/bin/failpoint-ctl
	@$(FAILPOINT_DISABLE)

tools/bin/failpoint-ctl: go.mod
	$(GO) build -o $@ github.com/pingcap/failpoint/failpoint-ctl

package: check
	pip install -r requirements.txt --target=deps
	GOOS=darwin GOARCH=amd64 go build -o bin/tiops ; \
	tar -czf tiops-$(VERSION)-darwin-amd64.tar.gz bin tiops deps tiops.py ; \
	shasum tiops-$(VERSION)-darwin-amd64.tar.gz | awk '{print $$1}' > tiops-$(VERSION)-darwin-amd64.sha1 ; \
	GOOS=linux GOARCH=amd64 go build -o bin/tiops ; \
	tar -czf tiops-$(VERSION)-linux-amd64.tar.gz bin tiops deps tiops.py ; \
	shasum tiops-$(VERSION)-linux-amd64.tar.gz | awk '{print $$1}' > tiops-$(VERSION)-linux-amd64.sha1 ; \
	rm bin/tiops

.PHONY: cmd package
