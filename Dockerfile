# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the SSH server binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /asteroids-ssh ./cmd/ssh

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS (if needed) and create non-root user
RUN apk add --no-cache ca-certificates && \
    adduser -D -s /bin/sh asteroids

WORKDIR /app

# Copy binary from builder
COPY --from=builder /asteroids-ssh /app/asteroids-ssh

# Create directory for host keys
RUN mkdir -p /app/keys && chown asteroids:asteroids /app/keys

# Switch to non-root user
USER asteroids

# Expose SSH port
EXPOSE 22

# Environment variables for configuration
ENV SSH_HOST=0.0.0.0
ENV SSH_PORT=22
ENV SSH_HOST_KEY=/app/keys/host_key

# Run the SSH server
CMD ["/app/asteroids-ssh"]
