CREATE TABLE IF NOT EXISTS gcfm_custom_fields (
    table_name VARCHAR(255) NOT NULL,
    column_name VARCHAR(255) NOT NULL,
    data_type VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (table_name, column_name)
);
CREATE TABLE IF NOT EXISTS gcfm_registry_schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE gcfm_registry_schema_version
    ADD COLUMN semver VARCHAR(32);
INSERT INTO gcfm_registry_schema_version(version, semver) VALUES (1, '0.1');
