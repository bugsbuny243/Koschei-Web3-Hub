#!/bin/sh
set -e

if [ "${ENABLE_WORKER:-false}" = "true" ]; then
  echo "starting Koschei worker in background"
  python3 /app/worker.py &
else
  echo "worker disabled; starting API only"
fi

exec /app/koschei-api
