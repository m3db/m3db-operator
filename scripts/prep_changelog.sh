#!/bin/bash

# Script for transforming non-hyperlink changelog entries into ones with
# hyperlinks using an opionated toolset (https://github.com/chmln/sd and
# https://github.com/BurntSushi/ripgrep).

PRS=$(mktemp)
NUMS=$(mktemp)

function cleanup() {
  rm -f "$PRS" "$NUMS"
}

trap cleanup EXIT

rg -o '\(#[0-9]+\)' CHANGELOG.md | while read -r PR; do
  NUM=$(<<<"$PR" rg -o '[0-9]+')
  echo "$NUM" >> "$NUMS"
  echo "[$NUM]: https://github.com/m3db/m3db-operator/pull/$NUM" >> "$PRS"
done

sort "$NUMS" | uniq | while read -r NUM; do
  sd -i "\(#${NUM}\)" "([#${NUM}][$NUM])" CHANGELOG.md
done

sort "$PRS" | uniq >> CHANGELOG.md
