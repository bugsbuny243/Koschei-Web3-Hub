#!/bin/sh
set -e

/app/koschei-api &
python3 /app/worker.py
