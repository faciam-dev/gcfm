ALTER TABLE gcfm_custom_fields
    ADD COLUMN IF NOT EXISTS store_kind TEXT NOT NULL DEFAULT 'sql',
    ADD COLUMN IF NOT EXISTS kind TEXT,
    ADD COLUMN IF NOT EXISTS physical_type TEXT,
    ADD COLUMN IF NOT EXISTS driver_extras JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE gcfm_custom_fields
SET kind = CASE LOWER(data_type)
    WHEN 'varchar'   THEN 'string'
    WHEN 'text'      THEN 'string'
    WHEN 'int'       THEN 'integer'
    WHEN 'integer'   THEN 'integer'
    WHEN 'bigint'    THEN 'integer'
    WHEN 'decimal'   THEN 'decimal'
    WHEN 'numeric'   THEN 'decimal'
    WHEN 'double'    THEN 'number'
    WHEN 'double precision' THEN 'number'
    WHEN 'date'      THEN 'datetime'
    WHEN 'datetime'  THEN 'datetime'
    WHEN 'timestamp' THEN 'datetime'
    WHEN 'json'      THEN 'object'
    WHEN 'jsonb'     THEN 'object'
    WHEN 'bytea'     THEN 'binary'
    WHEN 'blob'      THEN 'binary'
    ELSE 'any'
END
WHERE store_kind = 'mongo'
   OR LOWER(data_type) IN ('varchar','text','int','integer','bigint','decimal','numeric','double','double precision','date','datetime','timestamp','json','jsonb','bytea','blob');

UPDATE gcfm_custom_fields
SET physical_type = 'mongodb:' || kind
WHERE store_kind = 'mongo' AND (physical_type IS NULL OR physical_type = '');
