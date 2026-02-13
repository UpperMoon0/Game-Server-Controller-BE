package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/game-server/controller/pkg/config"
	_ "github.com/lib/pq" // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"go.uber.org/zap"
)

// Database wraps the SQL database connection
type Database struct {
	*sql.DB
	logger *zap.Logger
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.Config) (*Database, error) {
	var db *sql.DB
	var err error

	switch cfg.DatabaseType {
	case "sqlite":
		db, err = sql.Open("sqlite3", cfg.DatabaseHost)
	case "postgresql":
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseUser,
			cfg.DatabasePassword, cfg.DatabaseName, cfg.DatabaseSSLMode,
		)
		db, err = sql.Open("postgres", dsn)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.DatabaseType)
	}

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
