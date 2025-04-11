VERSION ?= 1.0.0
PROJECT_NAME = enemeter-data-processing

.PHONY: build build-all clean test

build:
	mkdir -p dist
	go build -o dist/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME)

build-all:
	mkdir -p dist
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o dist/$(PROJECT_NAME)-linux -ldflags="-X '$(PROJECT_NAME)/internal/commands.CurrentVersion=$(VERSION)'" ./cmd/$(PROJECT_NAME)
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build -o dist/$(PROJECT_NAME).exe -ldflags="-X '$(PROJECT_NAME)/internal/commands.CurrentVersion=$(VERSION)'" ./cmd/$(PROJECT_NAME)
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o dist/$(PROJECT_NAME)-mac -ldflags="-X '$(PROJECT_NAME)/internal/commands.CurrentVersion=$(VERSION)'" ./cmd/$(PROJECT_NAME)
	@echo "Build completed. Contents of dist/:"
	@ls -lh dist/

clean:
	rm -rf dist/

test:
	go test -v ./...
