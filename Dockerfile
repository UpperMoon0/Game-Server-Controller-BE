# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

#ARG for build secrets (not exposed in final image)
ARG DB_URL
ARG DB_USERNAME
ARG DB_PASSWORD
ARG DB_NAME

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

RUN apk --no-cache add ca-certificates curl

# Install Flyway with PostgreSQL JDBC driver
RUN curl -L -o /usr/local/flyway.tar.gz https://download.red-gate.com/flyway-commandline-10.4.1-linux-x64.tar.gz && \
    tar -xzf /usr/local/flyway.tar.gz -C /usr/local && \
    rm /usr/local/flyway.tar.gz && \
    ln -s /usr/local/flyway-10.4.1/flyway /usr/local/bin/flyway && \
    curl -L -o /usr/local/flyway-10.4.1/lib/postgresql.jar https://jdbc.postgresql.org/download/postgresql-42.7.1.jar

WORKDIR /app

# Set database environment variables from build args
ENV DATABASE_HOST=${DB_URL}
ENV DATABASE_USER=${DB_USERNAME}
ENV DATABASE_PASSWORD=${DB_PASSWORD}
ENV DATABASE_NAME=${DB_NAME}

# Copy binary from builder
COPY --from=builder /app/controller .
COPY --from=builder /app/config.yaml .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Create directories
RUN mkdir -p /app/data /app/logs

# Expose ports
EXPOSE 8080 50051

# Set environment variables
ENV CONFIG_PATH=/app/config.yaml

# Run Flyway migration on startup
CMD ["sh", "-c", "apk add --no-cache postgresql-client && psql -h $DATABASE_HOST -U $DATABASE_USER -c 'CREATE DATABASE $DATABASE_NAME;' 2>/dev/null || true && flyway -url=jdbc:postgresql://$DATABASE_HOST:5432/$DATABASE_NAME -user=$DATABASE_USER -password=$DATABASE_PASSWORD migrate && ./controller"]
