.PHONY: build build-proxy build-cli clean test

VERSION ?= 0.1.0
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build: build-proxy build-cli

build-proxy:
	go build $(LDFLAGS) -o dist/plasma-shield ./cmd/proxy

build-cli:
	go build $(LDFLAGS) -o dist/shield ./cmd/shield

clean:
	rm -rf dist/

test:
	go test ./...

# Build for multiple platforms
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/plasma-shield-linux-amd64 ./cmd/proxy
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/plasma-shield-linux-arm64 ./cmd/proxy
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/shield-darwin-amd64 ./cmd/shield
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/shield-darwin-arm64 ./cmd/shield
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/shield-linux-amd64 ./cmd/shield
