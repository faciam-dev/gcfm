#!/usr/bin/env bash
for i in {1..20}; do
  pg_isready -h localhost -p 5432 -U postgres && exit 0
  sleep 2
done
echo "Postgres did not become ready in time" >&2
exit 1
