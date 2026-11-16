#!/bin/bash
# Builds DXMT's d3d11.dll (x86_64 and i386) from the DXMT release Macron's
# runtime downloads (release.env), with the patches in runtime/dxmt/patches,
# using the MinGW compiler DXMT's own CI used for that release, and packs them
# as an overlay for the DXMT tree: lib/wine/<arch>-windows/d3d11.dll, plus the
# source of the patched file and the patches (DXMT is LGPL 2.1+). Built as
# DXMT's own build does, then `winebuild --builtin` (without that mark Wine
# does not take the DLL as its builtin), then stripped.
# Only the Windows side is rebuilt; DXMT's own winemetal.so (and its LLVM)
# stays as released, so the configure step gets an empty stand-in for DXMT's
# LLVM. Runs on a GitHub macOS runner with Homebrew.
# Usage: runtime/dxmt/build.sh <work-dir> <out-dir>
# DXMT_UNPATCHED=1 builds the same files without the patches, as a control
# for measuring what the patches change.
set -euo pipefail

work=$1
out=$2
here=$(cd "$(dirname "$0")" && pwd)
. "$here/release.env"

fetch() {
  local url=$1 sha=$2 path=$3
  [ -f "$path" ] && echo "$sha  $path" | shasum -a 256 -c --status && return
  curl -fsSL --retry 3 -o "$path" "$url"
  echo "$sha  $path" | shasum -a 256 -c
}

mkdir -p "$work" "$out"
work=$(cd "$work" && pwd)
out=$(cd "$out" && pwd)
cd "$work"

fetch "$LLVM_MINGW_URL" "$LLVM_MINGW_SHA256" llvm-mingw.tar.xz
rm -rf llvm-mingw && mkdir llvm-mingw
tar -xJf llvm-mingw.tar.xz -C llvm-mingw --strip-components 1
# DXMT's configure step needs the Wine import libraries (winecrt0, ntdll,
# dbghelp) its winemetal.dll links against; DXMT's CI takes them from this
# Wine build, and so does this script.
fetch "$DXMT_WINE_URL" "$DXMT_WINE_SHA256" dxmt-wine.tar.gz
rm -rf wine && mkdir wine
tar -xzf dxmt-wine.tar.gz -C wine
wine_tree=$work/wine

python3 -m venv tools
tools/bin/pip install --quiet "meson==$MESON_VERSION" "ninja==$NINJA_VERSION"
PATH="$work/tools/bin:$work/llvm-mingw/bin:$PATH"

rm -rf dxmt
git clone --quiet --depth 1 --branch "$DXMT_TAG" --recurse-submodules --shallow-submodules "$DXMT_REPO" dxmt
[ "$(git -C dxmt rev-parse HEAD)" = "$DXMT_COMMIT" ] || { echo "DXMT $DXMT_TAG is not $DXMT_COMMIT" >&2; exit 1; }
if [ "${DXMT_UNPATCHED:-0}" = 1 ]; then
  echo "DXMT_UNPATCHED=1: building without Macron's patches"
else
  for patch in "$here"/patches/*.patch; do
    echo "applying $(basename "$patch")"
    git -C dxmt apply --whitespace=nowarn "$patch"
  done
fi
mkdir -p no-llvm/include no-llvm/lib

build() { # <cross file> <arch>
  local dir=build-$2
  rm -rf "dxmt/$dir"
  (cd dxmt && meson setup --cross-file "$1" -Dnative_llvm_path="$work/no-llvm" -Dwine_install_path="$wine_tree" \
    --buildtype release "$dir" > "$work/setup-$2.log" 2>&1) || { tail -40 "$work/setup-$2.log"; exit 1; }
  ninja -C "dxmt/$dir" src/d3d11/d3d11.dll src/d3d11/d3d11.dll.postproc
  mkdir -p "$out/overlay/lib/wine/$2-windows"
  cp "dxmt/$dir/src/d3d11/d3d11.dll" "$out/overlay/lib/wine/$2-windows/d3d11.dll"
  llvm-strip "$out/overlay/lib/wine/$2-windows/d3d11.dll"
  grep -q 'Wine builtin DLL' "$out/overlay/lib/wine/$2-windows/d3d11.dll" || { echo "$2 d3d11.dll has no Wine builtin mark" >&2; exit 1; }
}

build build-win64.txt x86_64
build build-win32.txt i386

mkdir -p "$out/overlay/share/macron/dxmt"
cp "$here"/patches/*.patch dxmt/LICENSE "$out/overlay/share/macron/dxmt/"
cp dxmt/src/d3d11/d3d11_pipeline_cache.cpp "$out/overlay/share/macron/dxmt/"
{
  echo "DXMT $DXMT_TAG ($DXMT_COMMIT) with Macron's patches; $RELEASE_TAG"
  echo "built with $(x86_64-w64-mingw32-gcc --version | head -1), meson $MESON_VERSION"
} > "$out/overlay/share/macron/dxmt/VERSION"
tar -cJf "$out/$ASSET_NAME" -C "$out/overlay" .
(cd "$out" && shasum -a 256 "$ASSET_NAME" | tee "$ASSET_NAME.sha256")
