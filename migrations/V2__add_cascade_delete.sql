-- Flyway Migration: V2__add_cascade_delete.sql
-- Add ON DELETE CASCADE to foreign key constraints

-- Drop existing foreign key constraint on servers table
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_node_id_fkey;

-- Add new foreign key constraint with CASCADE DELETE
ALTER TABLE servers 
ADD CONSTRAINT servers_node_id_fkey 
FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE;

-- Drop existing foreign key constraint on node_events table
ALTER TABLE node_events DROP CONSTRAINT IF EXISTS node_events_node_id_fkey;

-- Add new foreign key constraint with CASCADE DELETE
ALTER TABLE node_events 
ADD CONSTRAINT node_events_node_id_fkey 
FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE;
