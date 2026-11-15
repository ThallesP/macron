#!/bin/sh
set -eu
here=$(cd "$(dirname "$0")" && pwd)
x86_64-w64-mingw32-gcc -O2 -Wall -mconsole "$here/probe.c" -o "$1/probe.exe" \
  -ld3d11 -ld3dcompiler_47 -ldxgi -ldxguid -luuid
