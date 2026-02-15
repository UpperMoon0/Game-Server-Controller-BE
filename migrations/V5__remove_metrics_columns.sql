-- V5: Remove runtime metrics columns from nodes table
-- These metrics are now tracked in-memory by the node agent and fetched via gRPC

ALTER TABLE nodes DROP COLUMN IF EXISTS player_count;
ALTER TABLE nodes DROP COLUMN IF EXISTS cpu_usage;
ALTER TABLE nodes DROP COLUMN IF EXISTS memory_usage;
ALTER TABLE nodes DROP COLUMN IF EXISTS uptime_seconds;
