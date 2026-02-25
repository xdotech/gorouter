.PHONY: build run test docker clean lint

BUILD_FLAGS := -ldflags="-s -w"
BINARY := gorouter

build:
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/gorouter

run: build
	PORT=14747 ./$(BINARY)

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

docker:
	docker build -t gorouter .

docker-run:
	docker run -d --name gorouter -p 14747:14747 \
		-v gorouter-data:/app/data \
		-e JWT_SECRET=change-me \
		-e INITIAL_PASSWORD=123456 \
		gorouter

docker-stop:
	docker stop gorouter && docker rm gorouter

clean:
	rm -f $(BINARY) coverage.out coverage.html

lint:
	@which golangci-lint > /dev/null || (echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run
