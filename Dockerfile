# Build stage
FROM golang:1.25-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/oszuidwest/zwfm-metadata"
LABEL org.opencontainers.image.description="Metadata handling middleware for ZuidWest FM"
LABEL org.opencontainers.image.licenses="MIT"

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download && go mod verify

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
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -extldflags '-static' -X zwfm-metadata/utils.Version=${VERSION} -X zwfm-metadata/utils.Commit=${COMMIT} -X 'zwfm-metadata/utils.BuildTime=${BUILD_TIME}'" \
    -a -installsuffix cgo \
    -o zwfm-metadata .

# Runtime stage
FROM alpine:3.22

# Install packages + create user + setup directories
RUN apk --no-cache add ca-certificates tzdata wget && \
    addgroup -g 1000 zwfm && \
    adduser -D -s /bin/sh -u 1000 -G zwfm zwfm && \
    mkdir -p /app && \
    chown zwfm:zwfm /app

WORKDIR /app

# Copy with correct ownership
COPY --from=builder --chown=zwfm:zwfm /app/zwfm-metadata ./zwfm-metadata
COPY --from=builder --chown=zwfm:zwfm /app/config-example.json ./config-example.json

# Switch to non-root user
USER zwfm

# Expose port
EXPOSE 9000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --timeout=3 -O /dev/null http://localhost:9000/ || exit 1

# Set default command
CMD ["./zwfm-metadata", "-config", "config.json"]