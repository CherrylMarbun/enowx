# The team cloud's server (docs/cloud.md). Built from the repository's
# root: docker build -f cmd/enowx-cloud/Dockerfile -t enowx-cloud .
# Built without SQLite: it needs ENOWX_CLOUD_DATABASE_URL, a Postgres URL.

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/root/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -tags nosqlite -trimpath -ldflags "-s -w" -o /enowx-cloud ./cmd/enowx-cloud

FROM alpine:3.22
RUN adduser -D -H -u 10001 cloud && mkdir /data && chown cloud /data
COPY --from=build /enowx-cloud /usr/local/bin/enowx-cloud
USER cloud
ENV ENOWX_CLOUD_ADDR=:8080 ENOWX_CLOUD_DATA=/data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["enowx-cloud"]
