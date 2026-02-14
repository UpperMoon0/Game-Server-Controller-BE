-- Flyway Migration: V3__simplify_nodes.sql
-- Simplify nodes table by removing unnecessary fields
-- Nodes are on the same machine as the controller, so hostname/IP are not needed
-- CPU/Memory/Storage/OS fields are removed as they are not essential

-- Step 1: Add new game_type column (single game type instead of array)
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS game_type VARCHAR(100);

-- Step 2: Migrate data from game_types to game_type (take first game type)
UPDATE nodes SET game_type = SUBSTRING(game_types::text FROM 2 FOR POSITION(',' IN game_types::text) - 2)
WHERE game_types::text LIKE '[%';
UPDATE nodes SET game_type = TRIM(BOTH '"' FROM game_types::text)
WHERE game_type IS NULL AND game_types::text LIKE '"%"';
UPDATE nodes SET game_type = game_types::text
WHERE game_type IS NULL AND game_types IS NOT NULL;

-- Step 3: Drop old columns that are no longer needed
ALTER TABLE nodes DROP COLUMN IF EXISTS hostname;
ALTER TABLE nodes DROP COLUMN IF EXISTS ip_address;
ALTER TABLE nodes DROP COLUMN IF EXISTS game_types;
ALTER TABLE nodes DROP COLUMN IF EXISTS total_cpu_cores;
ALTER TABLE nodes DROP COLUMN IF EXISTS total_memory_mb;
ALTER TABLE nodes DROP COLUMN IF EXISTS total_storage_mb;
ALTER TABLE nodes DROP COLUMN IF EXISTS available_cpu_cores;
ALTER TABLE nodes DROP COLUMN IF EXISTS available_memory_mb;
ALTER TABLE nodes DROP COLUMN IF EXISTS available_storage_mb;
ALTER TABLE nodes DROP COLUMN IF EXISTS os_version;

-- Step 4: Make game_type NOT NULL after migration
ALTER TABLE nodes ALTER COLUMN game_type SET NOT NULL;

-- Step 5: Set default port if not set
UPDATE nodes SET port = 8080 WHERE port IS NULL OR port = 0;
