#!/bin/sh
set -e

echo "Waiting for database to be ready..."
# Wait for PostgreSQL to be available
until PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -U "$DATABASE_USER" -d postgres -c '\q' 2>/dev/null; do
    echo "Database is unavailable - sleeping"
    sleep 2
done

echo "Database is ready!"

# Create database if it doesn't exist
echo "Creating database if not exists..."
PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -U "$DATABASE_USER" -d postgres -c "SELECT 1 FROM pg_database WHERE datname = '$DATABASE_NAME'" | grep -q 1 || \
    PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -U "$DATABASE_USER" -d postgres -c "CREATE DATABASE $DATABASE_NAME"

echo "Running database migrations..."

# Run Flyway migrations
flyway \
    -url="jdbc:postgresql://${DATABASE_HOST}:5432/${DATABASE_NAME}" \
    -user="$DATABASE_USER" \
    -password="$DATABASE_PASSWORD" \
    -locations="filesystem:/app/migrations" \
    migrate

echo "Migrations completed!"

# Start the controller
echo "Starting controller..."
exec ./controller "$@"
