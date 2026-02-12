# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go module files first
COPY go.mod ./

# Download dependencies and create go.sum
RUN go mod download

# Copy source code
COPY . .

# Get all dependencies to populate go.sum
RUN go get ./...

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o controller ./cmd/controller

# Production stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/controller .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/migrations ./migrations

# Create directories
RUN mkdir -p /app/data /app/logs

# Expose ports
EXPOSE 8080 50051

# Set environment variables
ENV CONFIG_PATH=/app/config.yaml

# Entry point
ENTRYPOINT ["./controller"]
