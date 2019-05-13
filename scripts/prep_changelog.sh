#!/bin/bash

PRS=$(mktemp)
NUMS=$(mktemp)

function cleanup() {
  rm -f "$PRS" "$NUMS"
}

trap cleanup EXIT

grep -Eo '\(#[0-9]+\)' CHANGELOG.md | while read -r PR; do
  NUM=$(<<<"$PR" grep -Eo '[0-9]+')
  echo "$NUM" >> "$NUMS"
  echo "[$NUM]: https://github.com/m3db/m3db-operator/pull/$NUM" >> "$PRS"
done

sort "$NUMS" | uniq | while read -r NUM; do
  EXPR="s@(#${NUM})@([#${NUM}][$NUM])@g"
  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i"" -e "$EXPR" CHANGELOG.md
  else
    sed -i "$EXPR" CHANGELOG.md
  fi
done

sort "$PRS" | uniq >> CHANGELOG.md
