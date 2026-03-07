APP_NAME := subscriptionlink
MAIN_PKG := ./cmd/server
DIST_DIR := dist
GOCACHE_DIR := $(CURDIR)/.gocache
CURRENT_PLATFORM := $(shell go env GOOS)/$(shell go env GOARCH)
comma := ,

# 默认只构建当前平台；可通过 PLATFORM 指定一个或多个目标（逗号分隔）
# 例如：
#   make build PLATFORM=linux/amd64
#   make build PLATFORM=linux/amd64,darwin/arm64,windows/amd64
TARGET_PLATFORMS := $(if $(PLATFORM),$(subst $(comma), ,$(PLATFORM)),$(CURRENT_PLATFORM))

.PHONY: build clean

build:
	@mkdir -p $(DIST_DIR)
	@mkdir -p $(GOCACHE_DIR)
	@echo "==> Target platforms: $(TARGET_PLATFORMS)"
	@for p in $(TARGET_PLATFORMS); do \
		GOOS=$${p%/*}; \
		GOARCH=$${p#*/}; \
		EXT=""; \
		if [ "$${GOOS}" = "windows" ]; then EXT=".exe"; fi; \
		OUT="$(DIST_DIR)/$(APP_NAME)-$${GOOS}-$${GOARCH}$${EXT}"; \
		echo "==> Building $${OUT}"; \
		CGO_ENABLED=0 GOCACHE="$(GOCACHE_DIR)" GOOS=$${GOOS} GOARCH=$${GOARCH} go build -o "$${OUT}" $(MAIN_PKG) || exit $$?; \
	done

clean:
	rm -rf $(DIST_DIR) $(GOCACHE_DIR)
