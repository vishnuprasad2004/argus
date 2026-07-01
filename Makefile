.PHONY: build run clean tidy

BINARY_NAME=argus
BUILD_DIR=bin

## Build the argus binary
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/

## Run argus directly
run:
	go run ./cmd/ $(ARGS)

## Tidy dependencies
tidy:
	go mod tidy

## Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

## Run all tests
test:
	go test ./... -v

## Build for all platforms
release:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/
