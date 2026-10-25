# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/users ./cmd/users && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/gateway ./cmd/gateway

RUN adduser -D -H -u 65532 appuser

FROM scratch

# TLS trust store, for verified PostgreSQL connections and any outbound HTTPS.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# So the numeric UID resolves to a name in logs.
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# One image builds both binaries; SERVICE picks which to run.
ARG SERVICE=gateway
COPY --from=builder /out/${SERVICE} /app/server

USER appuser:appuser

WORKDIR /app

ENV TZ=Europe/Moscow

EXPOSE 3000 9090

ENTRYPOINT ["/app/server"]
