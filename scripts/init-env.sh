#!/usr/bin/env bash
# Creates .env.local from .env.example with fresh random secrets, or, when it
# exists, adds only the variables it lacks. It never changes a value that is
# already set: the data key encrypts stored customer phone numbers, and
# replacing it would make them unreadable.
set -euo pipefail

target=".env.local"
example=".env.example"

# The value for a variable that must be a fresh secret, or nothing.
generated() {
  case "$1" in
    ZP_DATA_KEY | ZP_BLIND_INDEX_KEY | ZP_SIMULATOR_SECRET | ZP_PASSWORD_PEPPER)
      openssl rand -base64 32 ;;
    ZP_DEV_STAFF_PASSWORD) openssl rand -base64 18 ;;
  esac
}

# Prints an example line, filling in a secret when the value is empty.
render() {
  local key="${1%%=*}" value="${1#*=}"
  if [ -z "$value" ] && [ -n "$(generated "$key")" ]; then
    value="$(generated "$key")"
  fi
  printf '%s=%s\n' "$key" "$value"
}

is_variable() { [[ "$1" =~ ^[A-Z][A-Z0-9_]*= ]]; }

umask 077
if [ ! -e "$target" ]; then
  while IFS= read -r line; do
    if is_variable "$line"; then render "$line"; else printf '%s\n' "$line"; fi
  done < "$example" > "$target"
  echo "Created $target with new secrets. Keep it: it is ignored by git."
  exit 0
fi

added=0
while IFS= read -r line; do
  is_variable "$line" || continue
  key="${line%%=*}"
  grep -q "^${key}=" "$target" && continue
  [ "$added" -eq 0 ] && printf '\n# Added by make env\n' >> "$target"
  render "$line" >> "$target"
  echo "Added $key to $target"
  added=$((added + 1))
done < "$example"
[ "$added" -eq 0 ] && echo "$target already has every variable." >&2
exit 0
