ALTER TABLE custom_fields
    ADD COLUMN nullable BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN "unique" BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN "default" TEXT DEFAULT NULL,
    ADD COLUMN validator VARCHAR(64) DEFAULT NULL;
INSERT INTO registry_schema_version(version, semver) VALUES (6, '0.6');
