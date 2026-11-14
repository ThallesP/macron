#!/bin/bash
# Builds winemac.so (x86_64) from Wine 11.21 with the patches in
# runtime/wine/patches, checks that it is a drop-in for the winemac.so in
# Gcenx's wine-devel 11.21 build, and packs it as an overlay for that tree.
# Runs on an Apple silicon Mac (a GitHub macos-26 runner) with Xcode and
# Homebrew. Usage: runtime/wine/build-winemac.sh <work-dir> <out-dir>
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

brew list bison >/dev/null 2>&1 || brew install --quiet bison
PATH="$(brew --prefix bison)/bin:$PATH"

fetch "$WINE_SOURCE_URL" "$WINE_SOURCE_SHA256" wine-source.tar.gz
fetch "$GCENX_WINE_URL" "$GCENX_WINE_SHA256" gcenx-wine.tar.xz
fetch "$LLVM_MINGW_URL" "$LLVM_MINGW_SHA256" llvm-mingw.tar.xz
rm -rf llvm-mingw && mkdir llvm-mingw
tar -xJf llvm-mingw.tar.xz -C llvm-mingw --strip-components 1

rm -rf gcenx && mkdir gcenx
tar -xJf gcenx-wine.tar.xz -C gcenx --include '*/x86_64-unix/winemac.so' --include '*/x86_64-unix/win32u.so'
gcenx_winemac=$(find gcenx -path '*/x86_64-unix/winemac.so')
gcenx_win32u=$(find gcenx -path '*/x86_64-unix/win32u.so')
minos=$(otool -l "$gcenx_winemac" | awk '/LC_BUILD_VERSION/ {b=1} b && $1 == "minos" {print $2; exit}')
install_name=$(otool -D "$gcenx_winemac" | tail -n 1)
echo "Gcenx's winemac.so targets macOS $minos and is named $install_name"

rm -rf src build && mkdir src build
tar -xzf wine-source.tar.gz -C src --strip-components 1

configure() {
  (cd build && MACOSX_DEPLOYMENT_TARGET=$minos arch -x86_64 ../src/configure -C \
    --enable-archs=x86_64 --with-mingw="$work/llvm-mingw/bin/clang" \
    --disable-tests --disable-winebth_sys \
    --without-alsa --without-capi --with-coreaudio --without-cups --without-dbus \
    --without-ffmpeg --without-fontconfig --without-freetype --without-gettext \
    --without-gettextpo --without-gphoto --without-gnutls --without-gssapi \
    --without-gstreamer --without-inotify --without-krb5 --without-netapi \
    --without-opencl --without-opengl --without-oss --without-pcap --without-pcsclite \
    --with-pthread --without-pulse --without-sane --without-sdl --without-udev \
    --without-usb --without-v4l2 --without-vulkan --without-wayland --without-x \
    CFLAGS=-O2 LDFLAGS=-Wl,-rpath,@loader_path/../../ >configure.log 2>&1) || { tail -40 build/configure.log; exit 1; }
}

build() {
  (cd build && MACOSX_DEPLOYMENT_TARGET=$minos arch -x86_64 make -j"$(sysctl -n hw.ncpu)" dlls/winemac.drv/winemac.so dlls/win32u/win32u.so)
}

configure
build
cp build/dlls/winemac.drv/winemac.so "$work/winemac-upstream.so"
install_name_tool -id "$install_name" "$work/winemac-upstream.so"

for patch in "$here"/patches/*.patch; do
  patch -d src -p1 --forward <"$patch"
done
configure
build
cp build/dlls/winemac.drv/winemac.so "$work/winemac-patched.so"
install_name_tool -id "$install_name" "$work/winemac-patched.so"
cp build/dlls/win32u/win32u.so "$work/win32u-upstream.so"

"$here/check-abi.sh" "$gcenx_winemac" "$gcenx_win32u" "$work/winemac-upstream.so" "$work/win32u-upstream.so" "$work/winemac-patched.so" | tee "$out/abi-check.txt"

stage="$work/stage"
rm -rf "$stage" && mkdir -p "$stage/lib/wine/x86_64-unix" "$stage/share/doc/macron-wine"
cp "$work/winemac-patched.so" "$stage/lib/wine/x86_64-unix/winemac.so"
codesign --force --sign - "$stage/lib/wine/x86_64-unix/winemac.so"
cp "$here"/patches/*.patch "$here/patches/README.md" src/COPYING.LIB "$out/abi-check.txt" "$stage/share/doc/macron-wine/"
tar -C "$stage" -cJf "$out/$ASSET_NAME" lib share
cp "$here"/patches/*.patch "$out/"
(cd "$out" && shasum -a 256 "$ASSET_NAME" | tee "$ASSET_NAME.sha256")
