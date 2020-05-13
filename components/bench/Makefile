GOVER := $(shell go version)

GOOS    := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
GOARCH  := $(if $(GOARCH),$(GOARCH),amd64)
GOENV   := GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO      := $(GOENV) go
GOBUILD := $(GO) build $(BUILD_FLAG)
VERSION := "v0.0.2"

default: build

build: check
	$(GO) build -o bin/bench

lint:
	@golint ./...

vet:
	$(GO) vet ./...

check: lint vet

clean:
	@rm -rf bin

package: check
	GOOS=darwin GOARCH=amd64 go build ; \
	tar -czf bench-$(VERSION)-darwin-amd64.tar.gz bench ; \
	shasum bench-$(VERSION)-darwin-amd64.tar.gz | awk '{print $$1}' > bench-$(VERSION)-darwin-amd64.sha1 ; \
	GOOS=linux GOARCH=amd64 go build ; \
	tar -czf bench-$(VERSION)-linux-amd64.tar.gz bench; \
	shasum bench-$(VERSION)-linux-amd64.tar.gz | awk '{print $$1}' > bench-$(VERSION)-linux-amd64.sha1 ; \
	rm bench 

.PHONY: cmd package
