#!/usr/bin/env bash
# Creates and removes only its own isolated Calibre instance with synthetic data.
set -Eeuo pipefail
cd "$(dirname "$0")/.."
image="${PTC_TEST_IMAGE:-page-to-calibre:local}"
container="ptc-contract-${RANDOM}-${RANDOM}"
artifact_dir="$(mktemp -d -t ptc-contract-XXXXXX)"
container_id=""
cleanup() { if [[ -n "$container_id" ]]; then docker rm -f "$container_id" >/dev/null 2>&1 || true; fi; rm -rf "$artifact_dir"; }
trap cleanup EXIT
if ! docker image inspect "$image" >/dev/null 2>&1; then docker build -t "$image" .; fi
PTC_EPUB_ARTIFACT="$artifact_dir/probe.epub" mise exec go@1.27.3 -- go test ./internal/book -count=1
container_id="$(docker run -d --name "$container" --network none --entrypoint sleep "$image" infinity)"
docker exec -i "$container" sh -c 'cat > /tmp/probe.epub' < "$artifact_dir/probe.epub"
docker exec "$container" calibredb add --with-library /tmp/library /tmp/probe.epub
docker exec "$container" calibre-server --userdb /tmp/users.sqlite --manage-users -- add writer contract-test-only
docker exec "$container" calibre-server --userdb /tmp/users.sqlite --manage-users -- add reader contract-test-only --readonly
docker exec "$container" sh -c 'umask 077; printf %s contract-test-only > /tmp/password; printf %s wrong-test-password > /tmp/wrong-password'
docker exec -d "$container" sh -c 'exec calibre-server /tmp/library --port 8080 --enable-auth --userdb /tmp/users.sqlite > /tmp/server.log 2>&1'
export PTC_CONTRACT_CONTAINER="$container"
# Bounded CLI wait; each probe exercises the actual authenticated server.
ready=false
for attempt in {1..30}; do
  if docker exec "$container" calibredb list --with-library 'http://127.0.0.1:8080/#library' --username writer --password '<f:/tmp/password>' --for-machine >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ "$ready" != true ]]; then docker exec "$container" cat /tmp/server.log; exit 1; fi
mise exec go@1.27.3 -- go test ./internal/calibre -v -count=1
pnpm --dir web build
pnpm --dir web e2e
