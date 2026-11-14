#!/bin/bash
# Checks that a patched Wine PE module can replace the one in Gcenx's
# wine-devel 11.21 tree: an unpatched build of the same source and the patched
# build must export exactly what Gcenx's does.
# Usage: check-pe-exports.sh <llvm-objdump> <gcenx dll> <built dll> <patched dll>
set -euo pipefail

objdump=$1
gcenx=$2
upstream=$3
patched=$4
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

exports() { "$objdump" -p "$1" | awk '/^Export Table:/ {e=1; next} e && /^$/ {e=0} e && NF >= 3 && $1 ~ /^[0-9]+$/ {print $NF}' | sort; }

exports "$gcenx" >"$tmp/g"
exports "$upstream" >"$tmp/u"
exports "$patched" >"$tmp/p"
name=$(basename "$gcenx")
arch=$("$objdump" -f "$gcenx" | awk '/file format/ {print $NF}')
echo "== $name ($arch)"
if [ ! -s "$tmp/g" ]; then
  echo "DIFFERENT: found no exports in Gcenx's $name, the check is broken"
  exit 1
fi
if ! diff -u "$tmp/g" "$tmp/u"; then
  echo "FAIL: an unpatched build of $name exports something else than Gcenx's"
  exit 1
fi
if ! diff -u "$tmp/u" "$tmp/p"; then
  echo "FAIL: the patched $name exports something else than the unpatched build"
  exit 1
fi
echo "PASS: $name exports the same $(wc -l <"$tmp/g" | tr -d ' ') symbols as Gcenx's, patched and unpatched"
