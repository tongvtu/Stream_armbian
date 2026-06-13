# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy dependency definition and source files
COPY go.mod ./
COPY main.go ./
COPY templates/ ./templates/

# Compile the Go application with optimization flags (remove debug symbols to reduce size)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o stream-manager .

# Stage 2: Create runtime container
FROM debian:bookworm-slim

# Install standard ADB (android-tools-adb) and procps for system monitoring
RUN apt-get update && apt-get install -y --no-install-recommends \
    android-tools-adb \
    procps \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the compiled Go binary from the builder stage
COPY --from=builder /app/stream-manager .

# Expose the dashboard control port
EXPOSE 5000

# Run the manager daemon
CMD ["./stream-manager"]
