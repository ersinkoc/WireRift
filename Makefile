.PHONY: build test bench clean lint release

BINARY_SERVER=wirerift-server
BINARY_CLIENT=wirerift
VERSION?=1.0.0
GO=go
GOFLAGS=-v

build: build-server build-client

build-server:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_SERVER) ./cmd/wirerift-server

build-client:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_CLIENT) ./cmd/wirerift

test:
	$(GO) test $(GOFLAGS) ./...

bench:
	$(GO) test -bench=. -benchmem ./...

clean:
	rm -rf bin/
	rm -rf dist/

lint:
	$(GO) vet ./...

release:
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_SERVER)-linux-amd64 ./cmd/wirerift-server
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_CLIENT)-linux-amd64 ./cmd/wirerift
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_SERVER)-darwin-amd64 ./cmd/wirerift-server
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_CLIENT)-darwin-amd64 ./cmd/wirerift
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_SERVER)-darwin-arm64 ./cmd/wirerift-server
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_CLIENT)-darwin-arm64 ./cmd/wirerift
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_SERVER)-windows-amd64.exe ./cmd/wirerift-server
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" -o dist/$(BINARY_CLIENT)-windows-amd64.exe ./cmd/wirerift
