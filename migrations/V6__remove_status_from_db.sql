-- Remove status and last_heartbeat columns from nodes table
-- These are now fetched from node agents via gRPC, not stored in DB

ALTER TABLE nodes DROP COLUMN IF EXISTS status;
ALTER TABLE nodes DROP COLUMN IF EXISTS last_heartbeat;

-- Add a comment to explain the design decision
COMMENT ON TABLE nodes IS 'Node metadata only. Runtime state (status, last_heartbeat) is fetched from node agents via gRPC.';