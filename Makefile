GO ?= go
BIN := bin

.PHONY: build daemons test lint fmt run-app run-daemon clean

build: daemons
	$(GO) build -o $(BIN)/ ./cmd/...

# Linux builds of enowxd that the app installs on servers.
daemons:
	for arch in amd64 arm64; do \
		GOOS=linux GOARCH=$$arch CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o hostconn/daemons/enowxd-linux-$$arch ./cmd/enowxd && \
		gzip -9f hostconn/daemons/enowxd-linux-$$arch || exit 1; \
	done

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
