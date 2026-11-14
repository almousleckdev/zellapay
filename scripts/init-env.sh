#!/usr/bin/env bash
# Creates .env.local from .env.example with fresh random secrets.
# It refuses to overwrite an existing file: the data key encrypts stored
# customer phone numbers, and replacing it would make them unreadable.
set -euo pipefail

target=".env.local"
if [ -e "$target" ]; then
  echo "$target already exists; leaving it alone." >&2
  exit 0
fi

secret() { openssl rand -base64 32; }

umask 077
sed \
  -e "s|^ZP_DATA_KEY=.*|ZP_DATA_KEY=$(secret)|" \
  -e "s|^ZP_BLIND_INDEX_KEY=.*|ZP_BLIND_INDEX_KEY=$(secret)|" \
  -e "s|^ZP_SIMULATOR_SECRET=.*|ZP_SIMULATOR_SECRET=$(secret)|" \
  .env.example > "$target"

echo "Created $target with new secrets. Keep it: it is ignored by git."
