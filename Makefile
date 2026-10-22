GO ?= go
BIN := bin

.PHONY: build test lint fmt run-app run-daemon clean

build:
	$(GO) build -o $(BIN)/ ./cmd/...

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...
	$(GO) run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...

fmt:
	gofmt -w .

run-app:
	$(GO) run ./cmd/enowx

run-daemon:
	$(GO) run ./cmd/enowxd serve

clean:
	rm -rf $(BIN)
