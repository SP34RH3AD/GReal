BINARY  := greal
PKG     := .
DIST    := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# OS/ARCH targets: name = GOOS/GOARCH
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: all build run test vet fmt clean release $(PLATFORMS)

## build: compile for the host OS/ARCH
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

## run: build and show help
run: build
	./$(BINARY) --help

## test: run the test suite
test:
	go test ./...

## vet: static checks
vet:
	go vet ./...

## fmt: format the code
fmt:
	go fmt ./...

## release: cross-compile every platform into dist/
release: clean $(PLATFORMS)

# Per-platform build. $@ is e.g. "linux/amd64".
$(PLATFORMS):
	@os=$(word 1,$(subst /, ,$@)); arch=$(word 2,$(subst /, ,$@)); \
	ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
	out=$(DIST)/$(BINARY)-$$os-$$arch$$ext; \
	echo ">> $$out"; \
	GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
		go build -ldflags "$(LDFLAGS)" -o $$out $(PKG)

## clean: remove build artifacts
clean:
	rm -rf $(BINARY) $(DIST)
