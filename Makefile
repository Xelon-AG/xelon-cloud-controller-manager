# Project variables
PROJECT_NAME := xelon-cloud-controller-manager
IMAGE_NAME := xelonag/xelon-cloud-controller-manager

# Build variables
.DEFAULT_GOAL = test
BUILD_DIR := build
TOOLS_DIR := $(shell pwd)/tools
TOOLS_BIN_DIR := ${TOOLS_DIR}/bin
VERSION ?= $(shell git describe --always)


## tools: Install required tooling.
.PHONY: tools
tools:
	@echo "==> Installing required tooling..."
	@cd tools && GOBIN=${TOOLS_BIN_DIR} go install github.com/golangci/golangci-lint/cmd/golangci-lint


## clean: Delete the build directory.
.PHONY: clean
clean:
	@echo "==> Removing '$(BUILD_DIR)' directory..."
	@rm -rf $(BUILD_DIR)


## lint: Lint code with golangci-lint.
.PHONY: lint
lint:
	@echo "==> Linting code with 'golangci-lint'..."
	@${TOOLS_BIN_DIR}/golangci-lint run


## build: Build binary for linux/amd64 system.
.PHONE: build
build:
	@echo "==> Building binary..."
	@echo "    running go build for GOOS=linux GOARCH=amd64"
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o $(BUILD_DIR)/$(PROJECT_NAME) cmd/main.go


## test: Run all unit tests.
.PHONY: test
test:
	@echo "==> Running unit tests..."
	@mkdir -p $(BUILD_DIR)
	@go test -count=1 -v -cover -coverprofile=$(BUILD_DIR)/coverage.out -parallel=4 ./...


## build-docker-dev: Build docker dev image with included binary.
.PHONE: build-docker-dev
build-docker-dev: build
	@echo "==> Building docker image $(IMAGE_NAME):dev..."
	@docker build  --build-arg VERSION=$(VERSION) --tag $(IMAGE_NAME):dev --file Dockerfile build


## release-docker-dev: Release development docker image.
.PHONE: release-docker-dev
release-docker-dev: build-docker-dev
	@echo "==> Releasing development docker image $(IMAGE_NAME):dev..."
	@docker push $(IMAGE_NAME):dev


help: Makefile
	@echo "Usage: make <command>"
	@echo ""
	@echo "Commands:"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
