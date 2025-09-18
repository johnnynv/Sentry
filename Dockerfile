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
FROM ubuntu:22.04

# Install runtime dependencies (non-interactive)
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
RUN apt-get update && apt-get install -y \
    ca-certificates \
    git \
    curl \
    tzdata \
    bash \
    coreutils \
    && curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/ \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -s /bin/bash -m sentry

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
