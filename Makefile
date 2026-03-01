.PHONY: build run test docker clean lint up down logs

BUILD_FLAGS := -ldflags="-s -w"
BINARY := gorouter

# ─── Build & Run ─────────────────────────────────────────────────────────────

build:
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/gorouter

run: build
	PORT=14747 ./$(BINARY)

# ─── Test ────────────────────────────────────────────────────────────────────

test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ─── Docker ──────────────────────────────────────────────────────────────────

docker:
	docker build -t gorouter .

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f gorouter

# ─── Utilities ───────────────────────────────────────────────────────────────

clean:
	rm -f $(BINARY) coverage.out coverage.html

lint:
	@which golangci-lint > /dev/null || (echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run
