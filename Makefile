.PHONY: build build-router build-cli clean test

VERSION ?= 0.1.0
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build: build-router build-cli

build-router:
	go build $(LDFLAGS) -o dist/plasma-shield-router ./cmd/proxy

build-cli:
	go build $(LDFLAGS) -o dist/plasma-shield ./cmd/plasma-shield

clean:
	rm -rf dist/

test:
	go test ./...

# Build for multiple platforms
release:
	# Router (Linux only - runs on server)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/plasma-shield-router-linux-amd64 ./cmd/proxy
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/plasma-shield-router-linux-arm64 ./cmd/proxy
	# CLI (all platforms - runs on your machine)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/plasma-shield-darwin-amd64 ./cmd/plasma-shield
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/plasma-shield-darwin-arm64 ./cmd/plasma-shield
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/plasma-shield-linux-amd64 ./cmd/plasma-shield
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/plasma-shield-linux-arm64 ./cmd/plasma-shield
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/plasma-shield-windows-amd64.exe ./cmd/plasma-shield
