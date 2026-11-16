#!/bin/bash
# Checks that a patched Wine program can replace the one in Gcenx's
# wine-devel 11.21 tree: an unpatched build of the same source and the patched
# build must import exactly what Gcenx's does, so every function it calls is
# in the tree's modules. (A program exports nothing, so check-pe-exports.sh
# says nothing about it.)
# Usage: check-pe-imports.sh <llvm-objdump> <gcenx exe> <built exe> <patched exe>
set -euo pipefail

objdump=$1
gcenx=$2
upstream=$3
patched=$4
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

imports() {
  "$objdump" -p "$1" | awk '
    /DLL Name:/ {dll=tolower($NF); next}
    /^$/ {next}
    dll && /^ +[0-9]+ +[A-Za-z_][A-Za-z0-9_@?$]*$/ {print dll, $2}
  ' | sort -u
}

imports "$gcenx" >"$tmp/g"
imports "$upstream" >"$tmp/u"
imports "$patched" >"$tmp/p"
name=$(basename "$gcenx")
arch=$("$objdump" -f "$gcenx" | awk '/file format/ {print $NF}')
echo "== $name ($arch)"
if [ ! -s "$tmp/g" ]; then
  echo "DIFFERENT: found no imports in Gcenx's $name, the check is broken"
  exit 1
fi
if ! diff -u "$tmp/g" "$tmp/u"; then
  echo "FAIL: an unpatched build of $name imports something else than Gcenx's"
  exit 1
fi
if ! diff -u "$tmp/u" "$tmp/p"; then
  echo "FAIL: the patched $name imports something else than the unpatched build"
  exit 1
fi
echo "PASS: $name imports the same $(wc -l <"$tmp/g" | tr -d ' ') functions as Gcenx's, patched and unpatched"
