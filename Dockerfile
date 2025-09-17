# Multi-stage Dockerfile for Sentry - Tekton Pipeline Auto-Deployer

# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with version information
ARG VERSION=1.0.0
ARG BUILD_TIME
ARG GIT_COMMIT
ARG GIT_BRANCH

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w \
        -X 'main.Version=${VERSION}' \
        -X 'main.BuildTime=${BUILD_TIME}' \
        -X 'main.GitCommit=${GIT_COMMIT}' \
        -X 'main.GitBranch=${GIT_BRANCH}'" \
    -o sentry .

# Stage 2: Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    kubectl \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN adduser -D -s /bin/sh sentry

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/sentry /usr/local/bin/sentry

# Copy configuration files
COPY --from=builder /app/sentry.yaml /app/sentry.yaml
COPY --from=builder /app/env.example /app/env.example

# Create necessary directories
RUN mkdir -p /tmp/sentry && \
    chown -R sentry:sentry /app /tmp/sentry

# Switch to non-root user
USER sentry

# Set environment variables
ENV SENTRY_CONFIG_PATH="/app/sentry.yaml"
ENV SENTRY_TMP_DIR="/tmp/sentry"

# Expose health check port (if needed in future)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD sentry -action=validate > /dev/null || exit 1

# Default command
ENTRYPOINT ["sentry"]
CMD ["-action=watch", "-verbose"]
