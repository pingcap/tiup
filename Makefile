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

REPO := github.com/pingcap-incubator/tiup
LDFLAGS := -w -s
LDFLAGS += -X "$(REPO)/pkg/version.GitHash=$(COMMIT)"
LDFLAGS += -X "$(REPO)/pkg/version.GitBranch=$(BRANCH)"
LDFLAGS += -X "$(REPO)/pkg/version.BuildTime=$(BUILDTIME)"
LDFLAGS += $(EXTRA_LDFLAGS)

FILES     := $$(find . -name "*.go")

FAILPOINT_ENABLE  := $$(tools/bin/failpoint-ctl enable)
FAILPOINT_DISABLE := $$(tools/bin/failpoint-ctl disable)

default: cmd check

cmd:
	$(GOBUILD) -ldflags '$(LDFLAGS)' -o bin/tiup

check: fmt errcheck lint tidy check-static vet staticcheck

errcheck:tools/bin/errcheck
	@echo "errcheck"
	@GO111MODULE=on tools/bin/errcheck -exclude ./tools/check/errcheck_excludes.txt -ignoretests -blank $(PACKAGES)

check-static: tools/bin/golangci-lint
	tools/bin/golangci-lint run -v --disable-all --deadline=3m \
	  --enable=misspell \
	  --enable=ineffassign \
	  $$($(PACKAGE_DIRECTORIES))

lint:tools/bin/revive
	@echo "linting"
	@tools/bin/revive -formatter friendly -config tools/check/revive.toml $(FILES)

vet:
	$(GO) vet ./...

staticcheck:
	$(GO) get honnef.co/go/tools/cmd/staticcheck
	GO111MODULE=on staticcheck ./...

tidy:
	@echo "go mod tidy"
	./tools/check/check-tidy.sh

clean:
	@rm -rf bin

cover-dir:
	rm -rf cover
	mkdir -p cover

# Run tests
unit-test:
	TIUP_HOME=$(shell pwd)/tests/tiup_home $(GOTEST) ./... -covermode=count -coverprofile cover/cov.unit-test.out

integration_test:
	@$(GOTEST) -c -cover -covermode=count \
		-coverpkg=./... \
		-o tests/tiup_home/bin/tiup \
		github.com/pingcap-incubator/tiup/ ;
	@$(GOTEST) -c -cover -covermode=count \
			-coverpkg=./... \
			-o tests/tiup_home/bin/package ./components/package/ ;
	@$(GOTEST) -c -cover -covermode=count \
			-coverpkg=./... \
			-o tests/tiup_home/bin/playground ./components/playground/ ;
	@$(GOTEST) -c -cover -covermode=count \
			-coverpkg=./... \
			-o tests/tiup_home/bin/ctl ./components/ctl/ ;
	@$(GOTEST) -c -cover -covermode=count \
			-coverpkg=./... \
			-o tests/tiup_home/bin/doc ./components/doc/ ;
	@$(GOBUILD) -ldflags '$(LDFLAGS)' -o tests/tiup_home/bin/tiup.2
	@$(GOBUILD) -ldflags '$(LDFLAGS)' -o tests/tiup_home/bin/package ./components/package/
	@$(GOBUILD) -ldflags '$(LDFLAGS)' -o tests/tiup_home/bin/playground ./components/playground/
	@$(GOBUILD) -ldflags '$(LDFLAGS)' -o tests/tiup_home/bin/ctl ./components/ctl/
	@$(GOBUILD) -ldflags '$(LDFLAGS)' -o tests/tiup_home/bin/doc ./components/doc/
	cd tests && bash run.sh ; \


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

tiops:
	make -C components/tiops package

bench:
	make -C components/bench package

package: playground client pack tiops
	mkdir -p package ; \
	GOOS=darwin GOARCH=amd64 go build ; \
    tar -czf tiup-darwin-amd64.tar.gz tiup ; \
    shasum tiup-darwin-amd64.tar.gz | awk '{print $$1}' > tiup-darwin-amd64.sha1 ; \
    GOOS=linux GOARCH=amd64 go build ; \
    tar -czf tiup-linux-amd64.tar.gz tiup ; \
    shasum tiup-linux-amd64.tar.gz | awk '{print $$1}' > tiup-linux-amd64.sha1 ; \
    rm tiup ; \
    mv tiup* package/ ; \
    mv components/playground/playground-* package/ ; \
	mv components/client/client-* package/ ; \
	mv components/package/package-* package/ ; \
	mv components/tiops/tiops-* package/ ; \
    cp mirror/*.index package/
	cp install.sh package/

fmt:
	@echo "gofmt (simplify)"
	@gofmt -s -l -w $(FILES) 2>&1
	@echo "goimports (if installed)"
	$(shell gimports -w $(FILES) 2>/dev/null)

.PHONY: cmd package

tools/bin/errcheck: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/errcheck github.com/kisielk/errcheck

tools/bin/revive: tools/check/go.mod
	cd tools/check; \
	$(GO) build -o ../bin/revive github.com/mgechev/revive

tools/bin/golangci-lint:
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b ./tools/bin v1.21.0

