#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
flags="-O2 -Wall"
x86_64-w64-mingw32-gcc $flags -mconsole "$here/probe.c" -o "$1/probe64.exe" -ld3d9
if command -v i686-w64-mingw32-gcc > /dev/null; then
  i686-w64-mingw32-gcc $flags -mconsole "$here/probe.c" -o "$1/probe32.exe" -ld3d9
  exit 0
fi
uvx --from ziglang python -m ziglang cc -target x86-windows-gnu $flags "$here/probe.c" -o "$1/probe32.exe" -ld3d9
rm -f "$1/probe32.pdb"
