#!/bin/bash
# Builds Macron's Steam bridge from Valve's Proton source (fetch.sh) inside the
# Wine 11.21 tree that runtime/wine/build-overlay.sh builds Macron's Wine
# modules in, with the same toolchain and configure flags:
#   lib/wine/i386-windows/lsteamclient.dll    32-bit games
#   lib/wine/x86_64-windows/lsteamclient.dll  64-bit games
#   lib/wine/x86_64-unix/lsteamclient.so      loads the Mac client's steamclient.dylib
#   lib/wine/x86_64-windows/steam.exe         Proton's steam_helper, starts the game
# checks them, and packs them with their licences. Runs on an Apple silicon Mac
# (a GitHub macos-26 runner) with Xcode and Homebrew.
# Usage: lsteamclient/build.sh <work-dir> <out-dir>
set -euo pipefail

work=$1
out=$2
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
# Wine's source, Gcenx's build and llvm-mingw come from the overlay's pins; the
# bridge's own RELEASE_TAG and ASSET_NAME are read last and win.
. "$repo/runtime/wine/release.env"
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

brew list bison >/dev/null 2>&1 || brew install --quiet bison
PATH="$(brew --prefix bison)/bin:$PATH"

tree=$("$here/fetch.sh" "$work/proton")
fetch "$WINE_SOURCE_URL" "$WINE_SOURCE_SHA256" wine-source.tar.gz
fetch "$GCENX_WINE_URL" "$GCENX_WINE_SHA256" gcenx-wine.tar.xz
fetch "$LLVM_MINGW_URL" "$LLVM_MINGW_SHA256" llvm-mingw.tar.xz
rm -rf llvm-mingw && mkdir llvm-mingw
tar -xJf llvm-mingw.tar.xz -C llvm-mingw --strip-components 1
objdump="$work/llvm-mingw/bin/llvm-objdump"

rm -rf gcenx && mkdir gcenx
tar -xJf gcenx-wine.tar.xz -C gcenx --include '*/x86_64-unix/winemac.so' --include '*/x86_64-unix/ntdll.so'
gcenx_winemac=$(find gcenx -path '*/x86_64-unix/winemac.so')
gcenx_ntdll=$(find gcenx -path '*/x86_64-unix/ntdll.so')
minos=$(otool -l "$gcenx_winemac" | awk '/LC_BUILD_VERSION/ {b=1} b && $1 == "minos" {print $2; exit}')
echo "Gcenx's Wine targets macOS $minos"

rm -rf src build && mkdir src build
tar -xzf wine-source.tar.gz -C src --strip-components 1
cp -R "$tree/lsteamclient" src/dlls/lsteamclient
cp -R "$tree/steam_helper" src/programs/steam
awk '
  { print }
  $0 == "wine_fn_config_makefile dlls/localui/tests enable_tests" { print "wine_fn_config_makefile dlls/lsteamclient enable_lsteamclient"; found++ }
  $0 == "wine_fn_config_makefile programs/start enable_start" { print "wine_fn_config_makefile programs/steam enable_steam"; found++ }
  END { if (found != 2) { print "configure: the lines to add the bridge after were not found" > "/dev/stderr"; exit 1 } }
' src/configure >src/configure.bridge
mv src/configure.bridge src/configure
chmod +x src/configure

(cd build && MACOSX_DEPLOYMENT_TARGET=$minos arch -x86_64 ../src/configure -C \
  --enable-archs=i386,x86_64 --with-mingw="$work/llvm-mingw/bin/clang" \
  --disable-tests --disable-winebth_sys \
  --without-alsa --without-capi --with-coreaudio --without-cups --without-dbus \
  --without-ffmpeg --without-fontconfig --without-freetype --without-gettext \
  --without-gettextpo --without-gphoto --without-gnutls --without-gssapi \
  --without-gstreamer --without-inotify --without-krb5 --without-netapi \
  --without-opencl --without-opengl --without-oss --without-pcap --without-pcsclite \
  --with-pthread --without-pulse --without-sane --without-sdl --without-udev \
  --without-usb --without-v4l2 --without-vulkan --without-wayland --without-x \
  CFLAGS=-O2 CXXFLAGS=-O2 LDFLAGS=-Wl,-rpath,@loader_path/../../ >configure.log 2>&1) || { tail -40 build/configure.log; exit 1; }

targets=(dlls/lsteamclient/i386-windows/lsteamclient.dll dlls/lsteamclient/x86_64-windows/lsteamclient.dll
  dlls/lsteamclient/lsteamclient.so programs/steam/x86_64-windows/steam.exe)
(cd build && MACOSX_DEPLOYMENT_TARGET=$minos arch -x86_64 make -j"$(sysctl -n hw.ncpu)" "${targets[@]}")

report="$out/bridge-check.txt"
: >"$report"
check_pe() {
  local file=$1 format=$2 exports=$3 name
  "$objdump" -f "$file" | grep -q "file format $format" || { echo "$file is not $format" >&2; exit 1; }
  for name in $exports; do
    "$objdump" -p "$file" | grep -qw "$name" || { echo "$file does not export $name" >&2; exit 1; }
  done
  echo "$(basename "$(dirname "$file")")/$(basename "$file"): $format, $(stat -f %z "$file") bytes, exports $exports" | tee -a "$report"
}
check_pe build/dlls/lsteamclient/i386-windows/lsteamclient.dll coff-i386 "CreateInterface Steam_BGetCallback steamclient_init_registry"
check_pe build/dlls/lsteamclient/x86_64-windows/lsteamclient.dll coff-x86-64 "CreateInterface Steam_BGetCallback steamclient_init_registry"
check_pe build/programs/steam/x86_64-windows/steam.exe coff-x86-64 ""

unixlib=build/dlls/lsteamclient/lsteamclient.so
[ "$(lipo -archs "$unixlib")" = x86_64 ] || { echo "$unixlib is $(lipo -archs "$unixlib"), not x86_64" >&2; exit 1; }
nm -gU "$gcenx_ntdll" | awk '{print $NF}' | sort -u >ntdll-exports.txt
nm -m -u "$unixlib" | awk '/\(from ntdll\)/ {print $(NF-2)}' | sort -u >lsteamclient-ntdll-imports.txt
missing=$(comm -23 lsteamclient-ntdll-imports.txt ntdll-exports.txt)
[ -z "$missing" ] || { echo "lsteamclient.so needs symbols Gcenx's ntdll.so does not export: $missing" >&2; exit 1; }
echo "x86_64-unix/lsteamclient.so: x86_64, $(stat -f %z "$unixlib") bytes, its $(wc -l <lsteamclient-ntdll-imports.txt | tr -d ' ') ntdll.so imports are all exported by Gcenx's ntdll.so; links $(otool -L "$unixlib" | tail -n +2 | awk '{print $1}' | tr '\n' ' ')" | tee -a "$report"

stage="$work/stage"
doc="$stage/share/doc/macron-steam-bridge"
rm -rf "$stage" && mkdir -p "$stage/lib/wine/i386-windows" "$stage/lib/wine/x86_64-windows" "$stage/lib/wine/x86_64-unix" "$doc"
cp build/dlls/lsteamclient/i386-windows/lsteamclient.dll "$stage/lib/wine/i386-windows/"
cp build/dlls/lsteamclient/x86_64-windows/lsteamclient.dll "$stage/lib/wine/x86_64-windows/"
cp build/programs/steam/x86_64-windows/steam.exe "$stage/lib/wine/x86_64-windows/"
cp "$unixlib" "$stage/lib/wine/x86_64-unix/"
codesign --force --sign - "$stage/lib/wine/x86_64-unix/lsteamclient.so"
cp "$here/LICENSE" "$doc/LICENSE.lsteamclient"
cp "$repo/steam_helper/LICENSE" "$doc/LICENSE.steam_helper"
cp src/COPYING.LIB "$doc/COPYING.LIB.wine"
cp "$here/README.md" "$here/release.env" "$report" "$doc/"
cp "$here"/patches/*.patch "$repo"/steam_helper/patches/*.patch "$doc/"
tar -C "$stage" -cJf "$out/$ASSET_NAME" lib share
(cd "$out" && shasum -a 256 "$ASSET_NAME" | tee "$ASSET_NAME.sha256")
