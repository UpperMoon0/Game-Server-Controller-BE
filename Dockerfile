# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

#ARG for build secrets (not exposed in final image)
ARG DB_URL
ARG DB_USERNAME
ARG DB_PASSWORD

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

# Set database environment variables from build args
ENV DATABASE_HOST=${DB_URL}
ENV DATABASE_USER=${DB_USERNAME}
ENV DATABASE_PASSWORD=${DB_PASSWORD}

# Copy binary from builder
COPY --from=builder /app/controller .
COPY --from=builder /app/config.yaml .

# Copy migrations if they exist
RUN mkdir -p /app/migrations

# Create directories
RUN mkdir -p /app/data /app/logs

# Expose ports
EXPOSE 8080 50051

# Set environment variables
ENV CONFIG_PATH=/app/config.yaml

# Entry point
ENTRYPOINT ["./controller"]
