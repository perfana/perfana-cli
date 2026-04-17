#!/usr/bin/env bash
set -euo pipefail

# Keep the container alive when started without arguments so
# docker compose exec workflows can run commands later.
if [ "$#" -eq 0 ]; then
  exec sleep infinity
fi

# Allow callers to pass either:
#   1) perfana-cli <args>
#   2) <args> (implicitly for perfana-cli)
if [ "$1" = "perfana-cli" ]; then
  shift
fi

exec perfana-cli "$@"
