package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/game-server/controller/pkg/config"
	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
)

// Database wraps the SQL database connection
type Database struct {
	*sql.DB
	logger *zap.Logger
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.Config) (*Database, error) {
	dsn := cfg.GetDatabaseDSN()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{
		DB:     db,
		logger: zap.NewNop(),
	}, nil
}

// NewDatabaseWithLogger creates a database with a logger
func NewDatabaseWithLogger(db *sql.DB, logger *zap.Logger) *Database {
	return &Database{
		DB:     db,
		logger: logger,
	}
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.DB != nil {
		return d.DB.Close()
	}
	return nil
}
