# The team cloud's server (docs/cloud.md). Built from the repository's
# root: docker build -f cmd/enowx-cloud/Dockerfile -t enowx-cloud .
# Its data (SQLite) lives in /data: mount a volume there.

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
# Caches of their own (a build cache gone bad crashed the compiler), and
# two packages compiled at a time, for a small build server.
RUN --mount=type=cache,id=enowx-cloud-mod,target=/root/go/pkg/mod --mount=type=cache,id=enowx-cloud-build-2,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -p 2 -trimpath -ldflags "-s -w" -o /enowx-cloud ./cmd/enowx-cloud

FROM alpine:3.22
RUN adduser -D -H -u 10001 cloud && mkdir /data && chown cloud /data
COPY --from=build /enowx-cloud /usr/local/bin/enowx-cloud
USER cloud
ENV ENOWX_CLOUD_ADDR=:8080 ENOWX_CLOUD_DATA=/data
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["enowx-cloud"]
