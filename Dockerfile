# syntax=docker/dockerfile:1

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.23 AS builder

# Install build dependencies
RUN apk add --no-cache ca-certificates git

WORKDIR /src

# Copy go mod files
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy source and build
COPY . .

# Use Docker's automatic platform variables
ARG TARGETOS
ARG TARGETARCH

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME

# Build the binary for the target platform
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w -buildid= -X zwfm-metadata/utils.Version=${VERSION} -X zwfm-metadata/utils.Commit=${COMMIT} -X 'zwfm-metadata/utils.BuildTime=${BUILD_TIME}'" \
    -o /runtime/app/zwfm-metadata . && \
    cp config-example.json /runtime/app/config-example.json

# Runtime stage
# Chainguard static publishes only :latest (rolling) plus immutable digests; pin via digest in CI for reproducibility.
# hadolint ignore=DL3007
FROM cgr.dev/chainguard/static:latest

LABEL org.opencontainers.image.source="https://github.com/oszuidwest/zwfm-metadata"
LABEL org.opencontainers.image.description="Metadata handling middleware for ZuidWest FM"
LABEL org.opencontainers.image.licenses="MIT"

# Runtime files run as UID/GID 1000 to match volume permissions from prior alpine-based releases.
COPY --from=builder --chown=1000:1000 /runtime/app /app

WORKDIR /app
USER 1000:1000

# Expose port
EXPOSE 9000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/zwfm-metadata", "-healthcheck", "http://127.0.0.1:9000/"]

# Set default command
CMD ["/app/zwfm-metadata", "-config", "/app/config.json"]
