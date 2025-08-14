#!/usr/bin/env bash
set -euo pipefail

ALLOWLIST_PG=("authz.casbin_rule")
ALLOWLIST_MYSQL=("casbin_rule")

DB_DRIVER="${DB_DRIVER:-postgres}"
missing=""
case "$DB_DRIVER" in
  postgres)
    SQL=$(cat <<'SQL'
SELECT n.nspname||'.'||c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE c.relkind='r'
  AND n.nspname NOT IN ('pg_catalog','information_schema')
  AND (n.nspname||'.'||c.relname) NOT LIKE '%.gcfm_%';
SQL
)
    missing=$(psql "$TEST_DATABASE_URL" -At -c "$SQL" | sort -u)
    for ok in "${ALLOWLIST_PG[@]}"; do
      missing=$(echo "$missing" | grep -v -E "^${ok}$" || true)
    done
    ;;
  mysql)
    SQL=$(cat <<'SQL'
SELECT TABLE_NAME FROM information_schema.TABLES
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_TYPE='BASE TABLE'
    AND TABLE_NAME NOT LIKE 'gcfm\_%' ESCAPE '\';
SQL
)
    missing=$(mysql --batch --skip-column-names "$TEST_MYSQL_DSN" -e "$SQL" | sort -u)
    for ok in "${ALLOWLIST_MYSQL[@]}"; do
      missing=$(echo "$missing" | grep -v -E "^${ok}$" || true)
    done
    ;;
  *)
    echo "unknown DB_DRIVER: $DB_DRIVER" >&2
    exit 1
    ;;
esac

if [ -n "$missing" ]; then
  echo "❌ Non-prefixed tables found:"
  echo "$missing"
  echo "👉 Suggested rename SQL:"
  if [ "$DB_DRIVER" = "postgres" ]; then
    echo "$missing" | sed -E "s#^([^.]+)\.(.+)#ALTER TABLE \1.\"\2\" RENAME TO gcfm_\2;#"
  else
    echo "$missing" | sed -E "s#^(.+)#RENAME TABLE \1 TO gcfm_\1;#"
  fi
  exit 1
fi

echo "✅ Prefix check passed"
