#!/bin/bash
# Checks that a patched winemac.so can replace the one in Gcenx's wine-devel
# 11.21 tree: an unpatched winemac.so built here from the same source must
# import from Wine's own libraries, export, link and search exactly what
# Gcenx's does, and import nothing from the system that Gcenx's does not (a
# newer SDK may call ceil where an older compiler inlines it); the win32u.so
# it links against must export what Gcenx's does; and the patched one may only
# add _macdrv_functions. Prints a report and exits non-zero on any difference.
# Usage: check-abi.sh <gcenx winemac.so> <gcenx win32u.so> <built winemac.so>
#                     <built win32u.so> <patched winemac.so>
set -euo pipefail

gcenx=$1
gcenx_win32u=$2
upstream=$3
upstream_win32u=$4
patched=$5
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failed=0

imports() { nm -m "$1" | awk '$1 == "(undefined)" && $(NF-1) == "(from" {print $(NF-2), $NF}' | sort; }
wine_imports() { imports "$1" | grep -E '[ /](ntdll|win32u)(\.so)?\)$' || true; }
system_imports() { imports "$1" | grep -vE '[ /](ntdll|win32u)(\.so)?\)$' || true; }
rpaths() { otool -l "$1" | awk '$1 == "cmd" && $2 == "LC_RPATH" {r=1} r && $1 == "path" {print $2; r=0}'; }
exports() { nm -gU "$1" | awk '{print $NF}' | sort; }
dylibs() { otool -L "$1" | tail -n +2 | awk '{print $1}' | sort; }
text() { otool -s __TEXT __text "$1" | tail -n +3 | cut -f2-; }

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

echo "== Gcenx's winemac.so against an unpatched build of the same source"
wine_imports "$gcenx" >"$tmp/gi"; wine_imports "$upstream" >"$tmp/ui"
same "symbols imported from ntdll.so and win32u.so" "$tmp/gi" "$tmp/ui"
if [ ! -s "$tmp/gi" ]; then
  echo "DIFFERENT: found no symbols imported from ntdll.so or win32u.so, the check is broken"
  failed=1
fi
system_imports "$gcenx" >"$tmp/gs"; system_imports "$upstream" >"$tmp/us"
comm -13 "$tmp/gs" "$tmp/us" >"$tmp/extra"
if [ -s "$tmp/extra" ]; then
  echo "DIFFERENT: system symbols only the build here imports"
  cat "$tmp/extra"
  failed=1
else
  echo "ok: system symbols are a subset of Gcenx's ($(wc -l <"$tmp/us" | tr -d ' ') of $(wc -l <"$tmp/gs" | tr -d ' '); Gcenx's also imports: $(comm -23 "$tmp/gs" "$tmp/us" | awk '{print $1}' | tr '\n' ' '))"
fi
exports "$gcenx" >"$tmp/ge"; exports "$upstream" >"$tmp/ue"
same "exported symbols" "$tmp/ge" "$tmp/ue"
dylibs "$gcenx" >"$tmp/gl"; dylibs "$upstream" >"$tmp/ul"
same "install name and linked libraries" "$tmp/gl" "$tmp/ul"
rpaths "$gcenx" >"$tmp/gr"; rpaths "$upstream" >"$tmp/ur"
same "rpaths" "$tmp/gr" "$tmp/ur"
exports "$gcenx_win32u" >"$tmp/gw"; exports "$upstream_win32u" >"$tmp/uw"
same "win32u.so exported symbols" "$tmp/gw" "$tmp/uw"
text "$gcenx" >"$tmp/gt"; text "$upstream" >"$tmp/ut"
if cmp -s "$tmp/gt" "$tmp/ut"; then
  echo "note: __TEXT,__text is byte-identical"
else
  echo "note: __TEXT,__text differs ($(wc -l <"$tmp/gt" | tr -d ' ') vs $(wc -l <"$tmp/ut" | tr -d ' ') lines of 16 bytes); compilers differ"
fi
target() { otool -l "$1" | awk '/LC_BUILD_VERSION/ {b=1} b && $1 == "minos" {m=$2} b && $1 == "sdk" {print "minos " m ", sdk " $2; exit}'; }
echo "note: Gcenx's targets $(target "$gcenx"); ours $(target "$upstream")"

echo "== The patched winemac.so against the unpatched build"
imports "$upstream" >"$tmp/ua"; imports "$patched" >"$tmp/pa"
same "imported symbols" "$tmp/ua" "$tmp/pa"
exports "$patched" >"$tmp/pe"
{ cat "$tmp/ue"; echo _macdrv_functions; } | sort >"$tmp/want"
same "exported symbols (plus _macdrv_functions)" "$tmp/want" "$tmp/pe"
dylibs "$patched" >"$tmp/pl"
same "install name and linked libraries" "$tmp/ul" "$tmp/pl"
rpaths "$patched" >"$tmp/pr"
same "rpaths" "$tmp/ur" "$tmp/pr"

if [ "$failed" != 0 ]; then
  echo "FAIL: the patched winemac.so is not a drop-in for Gcenx's"
  exit 1
fi
echo "PASS: the patched winemac.so is a drop-in for Gcenx's"
