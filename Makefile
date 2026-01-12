BINARY_NAME=mono
INSTALL_PATH=$(HOME)/bin

VERSION_PKG=github.com/gwuah/mono/internal/version
BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS=-X $(VERSION_PKG).Branch=$(BRANCH) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)

.PHONY: build install clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/mono

install: build
	cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	rm -f $(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
