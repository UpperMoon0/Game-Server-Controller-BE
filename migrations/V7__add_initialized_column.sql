-- Add initialized column to nodes table
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS initialized BOOLEAN DEFAULT FALSE;

-- Update existing nodes to have initialized = true if they have a started_at date
UPDATE nodes SET initialized = true WHERE started_at IS NOT NULL;