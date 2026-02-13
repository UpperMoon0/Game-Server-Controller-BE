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

RUN apk --no-cache add ca-certificates postgresql-client curl

# Install Flyway with PostgreSQL JDBC driver
RUN curl -L -o /tmp/flyway.tar.gz "https://download.red-gate.com/flyway-commandline-10.4.1-linux-x64.tar.gz" && \
    tar -xzf /tmp/flyway.tar.gz -C /opt && \
    rm /tmp/flyway.tar.gz && \
    curl -L -o /opt/flyway-10.4.1/lib/postgresql.jar "https://jdbc.postgresql.org/download/postgresql-42.7.1.jar" && \
    ln -s /opt/flyway-10.4.1/flyway /usr/local/bin/flyway

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/controller .
COPY --from=builder /app/config.yaml .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Copy and set entrypoint script
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Create directories
RUN mkdir -p /app/data /app/logs

# Expose ports
EXPOSE 8080 50051

# Set environment variables
ENV CONFIG_PATH=/app/config.yaml

# Entry point - runs migration script then starts controller
ENTRYPOINT ["./entrypoint.sh"]
