.SILENT: ; # no need for @

PROJECT=acc
PROJECT_DIR=$(shell pwd)

GOFILES:=$(shell find . -name '*.go' -not -path './vendor/*')
GOPACKAGES:=$(shell go list ./... | grep -v /vendor/| grep -v /checkers)
OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

WORKDIR:=$(PROJECT_DIR)/_workdir

default: build-linux

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(WORKDIR)/$(PROJECT)_linux_amd64 $(GO_BUILD_FLAGS)

build:
	echo	CGO_ENABLED=0 go build -o $(WORKDIR)/$(PROJECT)_$(OS)_$(ARCH) $(GO_BUILD_FLAGS)

clean:
	rm -f $(WORKDIR)/*
	rm -rf .cover
	go clean -r

coverage:
	./_misc/coverage.sh

coverage-html:
	./_misc/coverage.sh --html

dependencies:
	go install github.com/mgechev/revive

develop: dependencies
	(cd .git/hooks && ln -sf ../../_misc/pre-push.bash pre-push )
	git flow init -d

lint:
	echo "revive..."
	revive $(GOPACKAGES)
	echo "go vet..."
	go vet --all $(GOPACKAGES)

test:
	CGO_ENABLED=0 go test $(GOPACKAGES)

test-race:
	CGO_ENABLED=1 go test -race $(GOPACKAGES)


