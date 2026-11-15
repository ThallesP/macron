#!/bin/bash
# prepare.sh <game-id> <exe> <title-regex>: make the prepared prefix that every
# measured run starts from: one start through macron (which creates and sets up
# the prefix), the game's window waited for, a clean `macron kill`
# (wineserver -k writes the registry), then a copy to ~/ct/prefix-ready/<game-id>.
set -u
gid=$1 exe=$2 title=$3
ct=$HOME/ct
prefix="$HOME/Library/Application Support/Macron/prefixes/$gid"
pkill -9 -f 'Macron/runtimes' 2>/dev/null
rm -rf "$prefix" "$ct/prefix-ready/$gid"
mkdir -p "$ct/prefix-ready"
"$ct/bin/macron" launch --game "$gid" --cwd "$(dirname "$exe")" -- "$exe" >"$ct/prepare.log" 2>&1 &
for i in $(seq 1 240); do
  sleep 1
  "$ct/scripts/macwin" windows | grep -q "\"title\":\"${title#^}" && break
  grep -q "exited with" "$ct/prepare.log" && break
done
sleep 10
"$ct/bin/macron" kill --game "$gid"
sleep 5
pkill -9 -f 'Macron/runtimes' 2>/dev/null
pkill -9 -f '^[A-Za-z]:\\' 2>/dev/null
sleep 1
cp -cR "$prefix" "$ct/prefix-ready/$gid"
du -sh "$ct/prefix-ready/$gid" >"$ct/prepare.done"
