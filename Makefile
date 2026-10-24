GO ?= go
BIN := bin
# Where `make dev` keeps its daemon, sessions, projects and hosts, apart
# from the installed app's ~/.enowx-app.
DEV_HOME ?= $(HOME)/.enowx-dev

.PHONY: build daemons dev test lint fmt run-app run-daemon clean package

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

# Every platform's installers, built here into dist/<version>, not published.
package:
	scripts/package.sh

# The app with hot reload: Go changes rebuild and relaunch it, and the
# relaunched app replaces the daemon an earlier build started. The Linux
# daemons are built first: the app embeds them to install on servers.
dev: daemons
	ENOWX_DEV=1 ENOWX_HOME=$(DEV_HOME) $(GO) tool mygo dev cmd/enowx

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
