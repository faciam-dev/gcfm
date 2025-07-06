#!/usr/bin/env bash
set -e
for i in {1..30}; do
  pg_isready -h localhost -p 5432 -U postgres && exit 0
  sleep 2
done
echo "Postgres not ready" >&2
exit 1
