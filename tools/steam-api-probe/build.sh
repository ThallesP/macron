#!/bin/bash
# Builds probe32.exe and probe64.exe and puts Valve's redistributable
# steam_api.dll and steam_api64.dll (Steamworks SDK 1.65, redistributable_bin)
# next to them in <out-dir>. The DLLs are fetched at a pinned commit of
# Steamworks.NET, which ships the SDK's redistributable_bin, and checked by
# SHA-256; they are never committed. <llvm-mingw-bin> is the bin folder of the
# llvm-mingw that runtime/wine/build-overlay.sh uses.
# Usage: tools/steam-api-probe/build.sh <llvm-mingw-bin> <out-dir>
set -euo pipefail

mingw=$1
out=$2
here=$(cd "$(dirname "$0")" && pwd)
redist=https://raw.githubusercontent.com/rlabrecque/Steamworks.NET/ea51b029a5dbcb47a99d3c1fac930b811672ed32/com.rlabrecque.steamworks.net/Plugins

mkdir -p "$out"
"$mingw/i686-w64-mingw32-clang" -O2 -Wall -mconsole "$here/probe.c" -o "$out/probe32.exe" -ladvapi32
"$mingw/x86_64-w64-mingw32-clang" -O2 -Wall -mconsole "$here/probe.c" -o "$out/probe64.exe" -ladvapi32

fetch() {
  local name=$1 sha=$2
  curl -fsSL --retry 3 -o "$out/$name" "$redist/$name"
  echo "$sha  $out/$name" | shasum -a 256 -c
}
fetch steam_api.dll 1cb8b7cb77923762d4e49b27e44295369a79e93b5d4282c5e840b268d28fe771
fetch steam_api64.dll e6d9bafb9a41e42fba7b21553db49f8719027af3f0deb23ff86cfc44d60e776d
