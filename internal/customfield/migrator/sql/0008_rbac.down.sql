DROP TABLE IF EXISTS gcfm_role_policies;
DROP TABLE IF EXISTS gcfm_user_roles;
DROP TABLE IF EXISTS gcfm_roles;
DELETE FROM registry_schema_version WHERE version=8;
