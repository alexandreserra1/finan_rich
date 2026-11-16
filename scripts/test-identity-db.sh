#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
container="rich-identity-test-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
docker run --detach --name "$container" --network none \
  --tmpfs /var/lib/postgresql/data:rw \
  --env POSTGRES_PASSWORD=senha-ficticia-de-teste \
  --mount "type=bind,src=$root/internal/identity,dst=/contrato,readonly" \
  postgres:16-alpine >/dev/null
attempt=0
until docker exec "$container" pg_isready -U postgres >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    printf '%s\n' 'O PostgreSQL de teste não ficou pronto.' >&2
    exit 1
  fi
  sleep 1
done
docker exec "$container" psql -X -U postgres -v ON_ERROR_STOP=1 -f /contrato/testdata/accounts_test.sql
