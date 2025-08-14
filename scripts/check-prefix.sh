#!/usr/bin/env bash
set -euo pipefail

ALLOWLIST_PG=("authz.casbin_rule")
ALLOWLIST_MYSQL=("casbin_rule")

DB_DRIVER="${DB_DRIVER:-postgres}"
missing=""
case "$DB_DRIVER" in
  postgres)
    SQL="SELECT n.nspname||'.'||c.relname\n         FROM pg_class c\n         JOIN pg_namespace n ON n.oid=c.relnamespace\n        WHERE c.relkind='r'\n          AND n.nspname NOT IN ('pg_catalog','information_schema')\n          AND (n.nspname||'.'||c.relname) NOT LIKE '%.gcfm_%';"
    missing=$(psql "$TEST_DATABASE_URL" -At -c "$SQL" | sort -u)
    for ok in "${ALLOWLIST_PG[@]}"; do
      missing=$(echo "$missing" | grep -v -E "^${ok}$" || true)
    done
    ;;
  mysql)
    SQL="SELECT TABLE_NAME FROM information_schema.TABLES\n          WHERE TABLE_SCHEMA = DATABASE()\n            AND TABLE_TYPE='BASE TABLE'\n            AND TABLE_NAME NOT LIKE 'gcfm\\_%' ESCAPE '\\';"
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
