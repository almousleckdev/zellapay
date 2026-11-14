#!/usr/bin/env bash
# Enforces file and line length limits. See CLAUDE.md, "Hard limits".
set -euo pipefail

MAX_FILE_LINES=300
MAX_GO_LINE=100
MAX_COMMENT_LINE=80
fail=0

files=$(git ls-files --cached --others --exclude-standard \
  | grep -Ev '(^|/)(go\.sum|\.DS_Store)$' || true)

for f in $files; do
  [ -f "$f" ] || continue
  if head -n 5 "$f" | grep -q 'Code generated'; then continue; fi
  lines=$(wc -l < "$f")
  if [ "$lines" -gt "$MAX_FILE_LINES" ]; then
    echo "FAIL $f: $lines lines (max $MAX_FILE_LINES)"; fail=1
  fi
  case "$f" in *.go)
    long=$(awk -v m="$MAX_GO_LINE" '{gsub(/\t/,"    ")} length($0)>m{c++} END{print c+0}' "$f")
    if [ "$long" -gt 0 ]; then
      echo "FAIL $f: $long lines over $MAX_GO_LINE chars"; fail=1
    fi
    longc=$(awk -v m="$MAX_COMMENT_LINE" '/^[ \t]*\/\//{gsub(/\t/,"    ")} /^[ \t]*\/\//&&length($0)>m{c++} END{print c+0}' "$f")
    if [ "$longc" -gt 0 ]; then
      echo "FAIL $f: $longc comment lines over $MAX_COMMENT_LINE chars"; fail=1
    fi ;;
  esac
done
exit $fail
