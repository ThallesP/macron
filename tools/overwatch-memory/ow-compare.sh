#!/bin/bash
# ow-compare.sh <label> <d3d11.dll | release> [seconds] [shell | play]
# Puts the given x86_64 d3d11.dll into the installed runtime's DXMT tree (or
# DXMT's released one, kept aside the first time this runs), records the
# SHA-256 of the file in the tree, and starts Overwatch with ow-run.sh: from a
# shell with Wine's loaddll channel on (the default; loaddll.txt shows which
# d3d11.dll Wine loaded), or with Steam's Play button. Needs DRIVE and
# EVIDENCE as ow-run.sh does, and Steam signed in.
set -u
label=$1 dll=$2 seconds=${3:-240} mode=${4:-shell}
here=$(cd "$(dirname "$0")" && pwd)
tree=$(ls -d "$HOME/Library/Application Support/Macron/runtimes/"*/wine-dxmt | head -1)
target="$tree/lib/wine/x86_64-windows/d3d11.dll"
release="$HOME/ow/d3d11-release.dll"
[ -f "$release" ] || cp "$target" "$release"
if [ "$dll" = release ]; then cp "$release" "$target"; else cp "$dll" "$target"; fi
out=$EVIDENCE/$label
mkdir -p "$out"
shasum -a 256 "$target" | sed "s|$HOME|~|" > "$out/d3d11.sha256"
if [ "$mode" = play ]; then
  "$here/ow-run.sh" "$label" play "$seconds"
else
  "$here/ow-run.sh" "$label" shell "$seconds" WINEDEBUG=-all,+loaddll
  grep -i 'd3d11\|winemetal' "$out/macron-stdout.txt" | head -10 > "$out/loaddll.txt"
fi
