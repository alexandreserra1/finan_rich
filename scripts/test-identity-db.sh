#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
container="rich-identity-test-$$"
results=$(mktemp -d "${TMPDIR:-/tmp}/rich-rate-test.XXXXXX")
cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$results"
}
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
docker exec "$container" psql -X -U postgres -v ON_ERROR_STOP=1 -f /contrato/testdata/rate_limits_test.sql
attempt=0
while [ "$attempt" -lt 20 ]; do
  attempt=$((attempt + 1))
  docker exec "$container" psql -X -U postgres -v ON_ERROR_STOP=1 -Atc \
    "SET ROLE rich_ratelimit; SELECT allowed::int FROM rich_private.consume_rate_limit('test-concurrency', repeat('c', 64));" > "$results/$attempt" &
done
wait
allowed=$(cat "$results"/* | awk '$0 == "1" { count++ } END { print count+0 }')
denied=$(cat "$results"/* | awk '$0 == "0" { count++ } END { print count+0 }')
if [ "$allowed" -ne 5 ] || [ "$denied" -ne 15 ]; then
  printf '%s\n' 'O limite compartilhado não respeitou 5 reservas em 20 chamadas concorrentes.' >&2
  exit 1
fi
printf '%s\n' 'Concorrência PostgreSQL: 5 chamadas permitidas, 15 recusadas.'
