.PHONY: frontend build package test clean version

APP := couswee
VERSION := v0.1.2
PKG := couswee/internal/version
GO ?= go
NPM ?= npm
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell TZ=Asia/Shanghai date '+%Y-%m-%dT%H:%M:%S%:z')
DIST_DIR := dist
PACKAGE_DIR := $(DIST_DIR)/package

LDFLAGS := -X '$(PKG).Version=$(VERSION)' \
	-X '$(PKG).Commit=$(COMMIT)' \
	-X '$(PKG).BuildTime=$(DATE)'

frontend:
	$(NPM) run build

build: frontend
	mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -tags embed_frontend -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(APP) ./cmd/couswee

package: build
	rm -rf $(PACKAGE_DIR)
	mkdir -p $(PACKAGE_DIR)
	cp $(DIST_DIR)/$(APP) $(PACKAGE_DIR)/$(APP)
	tar -C $(PACKAGE_DIR) -czf "$(DIST_DIR)/$(APP)-$(VERSION)-linux-amd64.tar.gz" $(APP)
	sha256sum "$(DIST_DIR)/$(APP)-$(VERSION)-linux-amd64.tar.gz" > "$(DIST_DIR)/$(APP)-$(VERSION)-linux-amd64.tar.gz.sha256"

test:
	$(NPM) test -- --run
	$(GO) test ./...

clean:
	rm -rf $(DIST_DIR)
	rm -f $(APP)

version:
	@echo $(VERSION)
