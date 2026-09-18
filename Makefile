.DEFAULT_GOAL := build

BIN_DIR := bin
MAN_DIR := man

ifeq ($(OS),Windows_NT)
	EXT := .exe
else
	EXT :=
endif

# Auto-discover all command directories under ./cmd
# BINARIES := $(notdir $(wildcard cmd/*))
BINARIES := $(filter-out vgmweb,$(notdir $(wildcard cmd/*)))

# If BINARY is set, only build that one; otherwise build all
ifdef BINARY
	TARGETS := $(BINARY)
else
	TARGETS := $(BINARIES)
endif

.PHONY: all build test test-race lint fmt fmt-check vet check clean install tools list pkg-docs man serve-blog utm

all: check

utm:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/utm ./cmd/utm

build: utm
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(MAN_DIR)
	@for b in $(TARGETS); do \
		echo "building $$b"; \
		go build -o $(BIN_DIR)/$$b$(EXT) ./cmd/$$b || exit 1; \
		echo "man page for $$b"; \
		$(BIN_DIR)/$$b -h 2>&1 | $(BIN_DIR)/utm $$b > $(MAN_DIR)/$$b.1 || exit 1; \
	done

vgmweb:
	GOOS=js GOARCH=wasm go build -o docs/vgmweb/vgmweb.wasm ./cmd/vgmweb
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" docs/vgmweb/wasm_exec.js

# Show what would be built
list:
	@echo $(BINARIES)

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	go fmt ./...

# CI-friendly gofmt check
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

vet:
	go vet ./...

lint:
	golangci-lint run

# CI-friendly check
check: fmt-check vet lint test

# Install all binaries into GOBIN
# install:
# 	@for b in $(BINARIES); do \
# 		echo "installing $$b"; \
# 		go install ./cmd/$$b || exit 1; \
# 	done

clean:
	rm -rf $(BIN_DIR)

tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

pkg-docs:
	GOPROXY=https://proxy.golang.org go list -m github.com/erik-adelbert/duh@main



serve-blog:
	@cd docs; \
	bundle exec jekyll serve; \
	cd -