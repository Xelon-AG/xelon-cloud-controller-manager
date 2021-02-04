# Project variables
PROJECT_NAME := xelon-cloud-controller-manager
IMAGE_NAME := xelonag/xelon-cloud-controller-manager

# Build variables
.DEFAULT_GOAL = test
BUILD_DIR := build


## tools: Install required tooling.
.PHONY: tools
tools:
ifeq (,$(wildcard ./.bin/golangci-lint*))
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b .bin/ v1.35.2
else
	@echo "==> Required tooling is already installed"
endif


## clean: Delete the build directory.
.PHONY: clean
clean:
	@echo "==> Removing '$(BUILD_DIR)' directory..."
	@rm -rf $(BUILD_DIR)


## lint: Lint code with golangci-lint.
.PHONY: lint
lint:
	@echo "==> Linting code with 'golangci-lint'..."
	@.bin/golangci-lint run ./...


## build: Build binary for linux/amd64 system.
.PHONE: build
build:
	@echo "==> Building binary..."
	@echo "    running go build for GOOS=linux GOARCH=amd64"
	@GOOS=linux GOARCH=amd64 go build -trimpath -o $(BUILD_DIR)/$(PROJECT_NAME) main.go


## test: Run all unit tests.
.PHONY: test
test:
	@echo "==> Running unit tests..."
	@mkdir -p $(BUILD_DIR)
	@go test -count=1 -v -cover -coverprofile=$(BUILD_DIR)/coverage.out -parallel=4 ./...


## build-docker: Build docker dev image with included binary.
.PHONE: build-docker
build-docker: build
	@echo "==> Building docker image $(IMAGE_NAME):dev..."
	@docker build -f Dockerfile.dev -t $(IMAGE_NAME):dev .


help: Makefile
	@echo "Usage: make <command>"
	@echo ""
	@echo "Commands:"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
