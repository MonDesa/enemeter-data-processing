PROJECT_NAME = enemeter-data-processing

.PHONY: build build-all clean test build-analyze

build:
	mkdir -p dist
	go build -o dist/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME)

build-analyze:
	mkdir -p dist
	go build -o dist/analyze-csv ./cmd/analyze-csv

build-all: build build-analyze
	mkdir -p dist
	@echo "Building $(PROJECT_NAME) for Linux..."
	GOOS=linux GOARCH=amd64 go build -o dist/$(PROJECT_NAME)-linux -ldflags="-X '$(PROJECT_NAME)/internal/commands.CurrentVersion=$(VERSION)'" ./cmd/$(PROJECT_NAME)
	@echo "Building $(PROJECT_NAME) for Windows..."
	GOOS=windows GOARCH=amd64 go build -o dist/$(PROJECT_NAME).exe -ldflags="-X '$(PROJECT_NAME)/internal/commands.CurrentVersion=$(VERSION)'" ./cmd/$(PROJECT_NAME)
	@echo "Building $(PROJECT_NAME) for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o dist/$(PROJECT_NAME)-mac -ldflags="-X '$(PROJECT_NAME)/internal/commands.CurrentVersion=$(VERSION)'" ./cmd/$(PROJECT_NAME)
	
	@echo "Building analyze-csv for Linux..."
	GOOS=linux GOARCH=amd64 go build -o dist/analyze-csv-linux ./cmd/analyze-csv
	@echo "Building analyze-csv for Windows..."
	GOOS=windows GOARCH=amd64 go build -o dist/analyze-csv.exe ./cmd/analyze-csv
	@echo "Building analyze-csv for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o dist/analyze-csv-mac ./cmd/analyze-csv
	
	@echo "Build completed. Contents of dist/:"
	@ls -lh dist/

clean:
	rm -rf dist/

test:
	go test -v ./...
