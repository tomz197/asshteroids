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

# Build the web server binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /asteroids-web ./cmd/web

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS (if needed) and create non-root user
RUN apk add --no-cache ca-certificates && \
    adduser -D -s /bin/sh asteroids

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /asteroids-ssh /app/asteroids-ssh
COPY --from=builder /asteroids-web /app/asteroids-web

# Copy startup script
COPY scripts/start.sh /app/start.sh

# Create directory for host keys and make script executable
RUN chmod +x /app/start.sh && \
    mkdir -p /app/keys && chown asteroids:asteroids /app/keys

# Switch to non-root user
USER asteroids

# Expose SSH and HTTP ports
EXPOSE 2222
EXPOSE 8080

# Environment variables for configuration
ENV SSH_HOST=0.0.0.0
ENV SSH_PORT=2222
ENV WEB_HOST=0.0.0.0
ENV WEB_PORT=8080
ENV SSH_DISPLAY_HOST=localhost

# Run both services
CMD ["/app/start.sh"]
