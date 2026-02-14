-- V4: Merge Server into Node

-- Add new columns to nodes table
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS version VARCHAR(50) DEFAULT '';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS player_count INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS cpu_usage DOUBLE PRECISION DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS memory_usage BIGINT DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS uptime_seconds BIGINT DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS started_at TIMESTAMP;

-- Update node status to include server statuses
-- Status values: installing, stopped, running, error, updating, starting, stopping, offline, maintenance

-- Drop the servers table (cascade will handle foreign keys)
DROP TABLE IF EXISTS servers;

-- Update existing nodes to have a default version
UPDATE nodes SET version = 'latest' WHERE version = '';
