#!/bin/sh
set -e

echo "Waiting for database to be ready..."

# Parse DB_URL to extract host and port (format: host:port)
DB_HOST=$(echo "$DB_URL" | cut -d':' -f1)
DB_PORT=$(echo "$DB_URL" | cut -d':' -f2)
DB_PORT=${DB_PORT:-5432}

# Wait for PostgreSQL to be available
until PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DATABASE_USER" -d postgres -c '\q' 2>/dev/null; do
    echo "Database is unavailable - sleeping"
    sleep 2
done

echo "Database is ready!"

# Create database if it doesn't exist
echo "Creating database if not exists..."
PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DATABASE_USER" -d postgres -c "SELECT 1 FROM pg_database WHERE datname = '$DATABASE_NAME'" | grep -q 1 || \
    PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DATABASE_USER" -d postgres -c "CREATE DATABASE $DATABASE_NAME"

echo "Running database migrations..."

# Run Flyway migrations
flyway \
    -url="jdbc:postgresql://${DB_HOST}:${DB_PORT}/${DATABASE_NAME}" \
    -user="$DATABASE_USER" \
    -password="$DATABASE_PASSWORD" \
    -locations="filesystem:/app/migrations" \
    migrate

echo "Migrations completed!"

# Start the controller
echo "Starting controller..."
exec ./controller "$@"
