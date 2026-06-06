BINARY := caltui
PKG    := ./cmd/caltui
BIN    := bin/$(BINARY)

.PHONY: build run install test lint fmt vet tidy update-golden etl clean

build: ## Build the binary into ./bin
	@mkdir -p bin
	go build -o $(BIN) $(PKG)

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
