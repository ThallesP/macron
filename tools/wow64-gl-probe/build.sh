#!/bin/sh
# Builds glprobe32.exe and glprobe64.exe into <out-dir>. Usage: build.sh <out-dir>
set -eu
here=$(cd "$(dirname "$0")" && pwd)
flags="-O2 -Wall"
x86_64-w64-mingw32-gcc $flags -mconsole "$here/glprobe.c" -o "$1/glprobe64.exe" -lopengl32 -lgdi32
if command -v i686-w64-mingw32-gcc > /dev/null; then
  i686-w64-mingw32-gcc $flags -mconsole "$here/glprobe.c" -o "$1/glprobe32.exe" -lopengl32 -lgdi32
  exit 0
fi
uvx --from ziglang python -m ziglang cc -target x86-windows-gnu $flags "$here/glprobe.c" -o "$1/glprobe32.exe" -lopengl32 -lgdi32
rm -f "$1/glprobe32.pdb"
