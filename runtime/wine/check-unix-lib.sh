#!/bin/bash
# Checks that a patched Wine Unix library other than winemac.so (opengl32.so)
# can replace the one in Gcenx's wine-devel 11.21 tree: an unpatched build of
# the same source must import from Wine's own libraries, export, link and
# search exactly what Gcenx's does and import nothing from the system that
# Gcenx's does not; the patched build must export, link and search what the
# unpatched one does and import nothing it does not. Its Windows side is
# shipped with it (check-pe-exports.sh), so the two always match.
# Usage: check-unix-lib.sh <gcenx lib.so> <built lib.so> <patched lib.so>
set -euo pipefail

gcenx=$1
upstream=$2
patched=$3
name=$(basename "$gcenx")
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failed=0

imports() { nm -m "$1" | awk '$1 == "(undefined)" && $(NF-1) == "(from" {print $(NF-2), $NF}' | sort; }
wine_imports() { imports "$1" | grep -E '[ /](ntdll|win32u)(\.so)?\)$' || true; }
system_imports() { imports "$1" | grep -vE '[ /](ntdll|win32u)(\.so)?\)$' || true; }
rpaths() { otool -l "$1" | awk '$1 == "cmd" && $2 == "LC_RPATH" {r=1} r && $1 == "path" {print $2; r=0}'; }
exports() { nm -gU "$1" | awk '{print $NF}' | sort; }
dylibs() { otool -L "$1" | tail -n +2 | awk '{print $1}' | sort; }

same() {
  local what=$1 a=$2 b=$3
  if diff -u "$a" "$b" >"$tmp/diff"; then
    echo "ok: $what ($(wc -l <"$a" | tr -d ' ') lines)"
  else
    echo "DIFFERENT: $what"
    cat "$tmp/diff"
    failed=1
  fi
}

subset() {
  local what=$1 a=$2 b=$3
  comm -13 "$a" "$b" >"$tmp/extra"
  if [ -s "$tmp/extra" ]; then
    echo "DIFFERENT: $what"
    cat "$tmp/extra"
    failed=1
  else
    echo "ok: $what"
  fi
}

echo "== Gcenx's $name against an unpatched build of the same source"
wine_imports "$gcenx" >"$tmp/gi"; wine_imports "$upstream" >"$tmp/ui"
same "symbols imported from ntdll.so and win32u.so" "$tmp/gi" "$tmp/ui"
if [ ! -s "$tmp/gi" ]; then
  echo "DIFFERENT: found no symbols imported from ntdll.so or win32u.so, the check is broken"
  failed=1
fi
system_imports "$gcenx" >"$tmp/gs"; system_imports "$upstream" >"$tmp/us"
subset "system symbols are a subset of Gcenx's" "$tmp/gs" "$tmp/us"
exports "$gcenx" >"$tmp/ge"; exports "$upstream" >"$tmp/ue"
same "exported symbols" "$tmp/ge" "$tmp/ue"
dylibs "$gcenx" >"$tmp/gl"; dylibs "$upstream" >"$tmp/ul"
same "install name and linked libraries" "$tmp/gl" "$tmp/ul"
rpaths "$gcenx" >"$tmp/gr"; rpaths "$upstream" >"$tmp/ur"
same "rpaths" "$tmp/gr" "$tmp/ur"

echo "== The patched $name against the unpatched build"
imports "$upstream" >"$tmp/ua"; imports "$patched" >"$tmp/pa"
subset "imported symbols are a subset of the unpatched build's" "$tmp/ua" "$tmp/pa"
exports "$patched" >"$tmp/pe"
same "exported symbols" "$tmp/ue" "$tmp/pe"
dylibs "$patched" >"$tmp/pl"
same "install name and linked libraries" "$tmp/ul" "$tmp/pl"
rpaths "$patched" >"$tmp/pr"
same "rpaths" "$tmp/ur" "$tmp/pr"

if [ "$failed" != 0 ]; then
  echo "FAIL: the patched $name is not a drop-in for Gcenx's"
  exit 1
fi
echo "PASS: the patched $name is a drop-in for Gcenx's"
