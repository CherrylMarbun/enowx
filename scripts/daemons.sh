#!/bin/sh
# Builds the Linux enowxd the app embeds, to install on servers.
set -eu
cd "$(dirname "$0")/.."
go=${GO:-go}
for arch in amd64 arm64; do
	GOOS=linux GOARCH=$arch CGO_ENABLED=0 $go build -trimpath -ldflags "-s -w" -o hostconn/daemons/enowxd-linux-$arch ./cmd/enowxd
	gzip -9f hostconn/daemons/enowxd-linux-$arch
done
