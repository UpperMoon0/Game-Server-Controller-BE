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

# Download and run Flyway
FLYWAY_VERSION="10.4.1"
curl -L -o /tmp/flyway.tar.gz "https://download.red-gate.com/flyway-commandline-${FLYWAY_VERSION}-linux-x64.tar.gz"
tar -xzf /tmp/flyway.tar.gz -C /tmp
rm /tmp/flyway.tar.gz

# Download PostgreSQL JDBC driver
curl -L -o /tmp/flyway/lib/postgresql.jar "https://jdbc.postgresql.org/download/postgresql-42.7.1.jar"

# Run Flyway migrations
/tmp/flyway-${FLYWAY_VERSION}/flyway \
    -url="jdbc:postgresql://${DATABASE_HOST}:5432/${DATABASE_NAME}" \
    -user="$DATABASE_USER" \
    -password="$DATABASE_PASSWORD" \
    -locations="filesystem:/app/migrations" \
    migrate

echo "Migrations completed!"

# Start the controller
echo "Starting controller..."
exec ./controller "$@"
