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

// Migrate runs database migrations
func (d *Database) Migrate() error {
	migrations := []string{
		// Nodes table
		`CREATE TABLE IF NOT EXISTS nodes (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			hostname VARCHAR(255) NOT NULL,
			ip_address VARCHAR(45) NOT NULL,
			port INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'unknown',
			game_types TEXT NOT NULL,
			total_cpu_cores INTEGER NOT NULL,
			total_memory_mb BIGINT NOT NULL,
			total_storage_mb BIGINT NOT NULL,
			available_cpu_cores INTEGER NOT NULL,
			available_memory_mb BIGINT NOT NULL,
			available_storage_mb BIGINT NOT NULL,
			os_version VARCHAR(100),
			agent_version VARCHAR(50),
			heartbeat_interval INTEGER DEFAULT 30,
			last_heartbeat TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Servers table
		`CREATE TABLE IF NOT EXISTS servers (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			node_id VARCHAR(36) NOT NULL REFERENCES nodes(id),
			game_type VARCHAR(100) NOT NULL,
			instance_id VARCHAR(36) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'stopped',
			version VARCHAR(50) NOT NULL,
			settings TEXT NOT NULL,
			env_vars TEXT NOT NULL,
			max_players INTEGER DEFAULT 32,
			world_name VARCHAR(255),
			online_mode BOOLEAN DEFAULT TRUE,
			port INTEGER NOT NULL,
			query_port INTEGER NOT NULL,
			rcon_port INTEGER NOT NULL,
			ip_address VARCHAR(45) NOT NULL,
			player_count INTEGER DEFAULT 0,
			cpu_usage REAL DEFAULT 0,
			memory_usage BIGINT DEFAULT 0,
			uptime_seconds BIGINT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMP
		)`,

		// Node events table
		`CREATE TABLE IF NOT EXISTS node_events (
			id VARCHAR(36) PRIMARY KEY,
			node_id VARCHAR(36) NOT NULL REFERENCES nodes(id),
			type VARCHAR(50) NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			data TEXT
		)`,

		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_servers_node_id ON servers(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_servers_game_type ON servers(game_type)`,
		`CREATE INDEX IF NOT EXISTS idx_node_events_node_id ON node_events(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_node_events_timestamp ON node_events(timestamp)`,
	}

	for _, migration := range migrations {
		if _, err := d.Exec(migration); err != nil {
			return fmt.Errorf("failed to run migration: %w", err)
		}
	}

	return nil
}
