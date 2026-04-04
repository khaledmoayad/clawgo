VERSION ?= dev
BINARY := clawgo
MODULE := github.com/khaledmoayad/clawgo
LDFLAGS := -s -w -X $(MODULE)/internal/cli.Version=$(VERSION)

.PHONY: build test test-race lint install clean

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/clawgo

test:
	go test -v ./...

test-race:
	CGO_ENABLED=0 go test -race -count=1 ./...

lint:
	golangci-lint run ./...

install:
	go install -ldflags="$(LDFLAGS)" ./cmd/clawgo

clean:
	rm -rf bin/
