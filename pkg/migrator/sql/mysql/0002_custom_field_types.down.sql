ALTER TABLE gcfm_custom_fields
    DROP COLUMN IF EXISTS driver_extras,
    DROP COLUMN IF EXISTS physical_type,
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS store_kind;
