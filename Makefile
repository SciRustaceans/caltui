BINARY := caltui
PKG    := ./cmd/caltui
BIN    := bin/$(BINARY)

.PHONY: build run install test lint fmt vet tidy update-golden etl clean dist

build: ## Build the binary into ./bin (native OS/arch)
	@mkdir -p bin
	go build -o $(BIN) $(PKG)

dist: ## Cross-compile release binaries for macOS + Linux + Windows (pure Go, no CGo)
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64      $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64      $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o dist/$(BINARY)-linux-arm64       $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64       $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o dist/$(BINARY)-windows-arm64.exe $(PKG)

run: ## Run the app
	go run $(PKG)

install: ## Install to GOBIN (~/go/bin)
	go install $(PKG)

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format code
	gofmt -s -w .

vet: ## go vet
	go vet ./...

tidy: ## Tidy module deps
	go mod tidy

update-golden: ## Regenerate teatest golden snapshots
	go test ./... -update

etl: ## Rebuild the bundled USDA food database (data/foods.sqlite.gz)
	go run ./tools/etl

clean: ## Remove build artifacts
	rm -rf bin dist
