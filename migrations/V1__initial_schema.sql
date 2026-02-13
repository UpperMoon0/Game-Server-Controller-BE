-- Flyway Migration: V1__initial_schema.sql
-- Game Server Controller Database Migration
-- This script creates the database and required tables

-- Nodes table
CREATE TABLE IF NOT EXISTS nodes (
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
);

-- Servers table
CREATE TABLE IF NOT EXISTS servers (
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
);

-- Node events table
CREATE TABLE IF NOT EXISTS node_events (
    id VARCHAR(36) PRIMARY KEY,
    node_id VARCHAR(36) NOT NULL REFERENCES nodes(id),
    type VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    data TEXT
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_servers_node_id ON servers(node_id);
CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status);
CREATE INDEX IF NOT EXISTS idx_servers_game_type ON servers(game_type);
CREATE INDEX IF NOT EXISTS idx_node_events_node_id ON node_events(node_id);
CREATE INDEX IF NOT EXISTS idx_node_events_timestamp ON node_events(timestamp);
