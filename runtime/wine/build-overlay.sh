#!/bin/bash
# Builds winemac.so (x86_64), wined3d.dll (i386 and x86_64), wow64cpu.dll
# (x86_64), opengl32 (opengl32.so, x86_64, and opengl32.dll, i386 and
# x86_64), wineboot.exe (x86_64), avrt.dll (i386 and x86_64) and wineserver
# (x86_64) from Wine 11.21 with the patches in runtime/wine/patches, checks
# that they are drop-ins for the same files in Gcenx's wine-devel 11.21 build,
# and packs them as an overlay for that tree. Runs on an Apple silicon Mac (a GitHub macos-26
# runner) with Xcode and Homebrew. Usage: runtime/wine/build-overlay.sh <work-dir> <out-dir>
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
tar -xJf gcenx-wine.tar.xz -C gcenx --include '*/x86_64-unix/winemac.so' --include '*/x86_64-unix/win32u.so' \
  --include '*/i386-windows/wined3d.dll' --include '*/x86_64-windows/wined3d.dll' --include '*/x86_64-windows/wow64cpu.dll' \
  --include '*/x86_64-unix/opengl32.so' --include '*/i386-windows/opengl32.dll' --include '*/x86_64-windows/opengl32.dll' \
  --include '*/x86_64-windows/wineboot.exe' --include '*/i386-windows/avrt.dll' --include '*/x86_64-windows/avrt.dll' \
  --include '*/bin/wineserver' --include '*/lib/libinotify.0.dylib'
gcenx_winemac=$(find gcenx -path '*/x86_64-unix/winemac.so')
gcenx_win32u=$(find gcenx -path '*/x86_64-unix/win32u.so')
gcenx_opengl32=$(find gcenx -path '*/x86_64-unix/opengl32.so')
gcenx_wineserver=$(find gcenx -path '*/bin/wineserver')
wineserver_inotify=$(otool -L "$gcenx_wineserver" | awk '/inotify/ {print $1}')
wineserver_rpath=$(otool -l "$gcenx_wineserver" | awk '$1 == "cmd" && $2 == "LC_RPATH" {r=1} r && $1 == "path" {print $2; exit}')
echo "Gcenx's wineserver links $wineserver_inotify and searches $wineserver_rpath"
rm -rf inotify && mkdir -p inotify/include/sys inotify/lib
fetch "$INOTIFY_HEADER_URL" "$INOTIFY_HEADER_SHA256" inotify/include/sys/inotify.h
cp "$(find gcenx -path '*/lib/libinotify.0.dylib')" inotify/lib/libinotify.dylib
minos=$(otool -l "$gcenx_winemac" | awk '/LC_BUILD_VERSION/ {b=1} b && $1 == "minos" {print $2; exit}')
install_name=$(otool -D "$gcenx_winemac" | tail -n 1)
opengl32_install_name=$(otool -D "$gcenx_opengl32" | tail -n 1)
echo "Gcenx's winemac.so targets macOS $minos and is named $install_name; opengl32.so is named $opengl32_install_name"

rm -rf src build && mkdir src build
tar -xzf wine-source.tar.gz -C src --strip-components 1

configure() {
  (cd build && MACOSX_DEPLOYMENT_TARGET=$minos arch -x86_64 ../src/configure -C \
    --enable-archs=i386,x86_64 --with-mingw="$work/llvm-mingw/bin/clang" \
    --disable-tests --disable-winebth_sys \
    --without-alsa --without-capi --with-coreaudio --without-cups --without-dbus \
    --without-ffmpeg --without-fontconfig --without-freetype --without-gettext \
    --without-gettextpo --without-gphoto --without-gnutls --without-gssapi \
    --without-gstreamer --with-inotify --without-krb5 --without-netapi \
    --without-opencl --without-opengl --without-oss --without-pcap --without-pcsclite \
    --with-pthread --without-pulse --without-sane --without-sdl --without-udev \
    --without-usb --without-v4l2 --without-vulkan --without-wayland --without-x \
    INOTIFY_CFLAGS="-I$work/inotify/include" INOTIFY_LIBS="-L$work/inotify/lib -linotify" \
    CFLAGS=-O2 LDFLAGS=-Wl,-rpath,@loader_path/../../ >configure.log 2>&1) || { tail -40 build/configure.log; exit 1; }
  grep -q '^#define HAVE_SYS_INOTIFY_H 1' build/include/config.h || { echo "configure did not find the inotify library"; grep -i inotify build/configure.log | tail -5; exit 1; }
}

build() {
  (cd build && MACOSX_DEPLOYMENT_TARGET=$minos arch -x86_64 make -j"$(sysctl -n hw.ncpu)" dlls/winemac.drv/winemac.so dlls/win32u/win32u.so \
    dlls/wined3d/i386-windows/wined3d.dll dlls/wined3d/x86_64-windows/wined3d.dll dlls/wow64cpu/x86_64-windows/wow64cpu.dll \
    dlls/opengl32/opengl32.so dlls/opengl32/i386-windows/opengl32.dll dlls/opengl32/x86_64-windows/opengl32.dll \
    programs/wineboot/x86_64-windows/wineboot.exe dlls/avrt/i386-windows/avrt.dll dlls/avrt/x86_64-windows/avrt.dll \
    server/wineserver)
}

# Gcenx's wineserver finds libinotify in the tree's lib folder, next to bin.
keep_wineserver() { # <upstream|patched>
  cp build/server/wineserver "$work/wineserver-$1"
  local linked
  linked=$(otool -L "$work/wineserver-$1" | awk '/inotify/ {print $1}')
  [ "$linked" = "$wineserver_inotify" ] || install_name_tool -change "$linked" "$wineserver_inotify" "$work/wineserver-$1"
  install_name_tool -rpath @loader_path/../../ "$wineserver_rpath" "$work/wineserver-$1"
}

keep_opengl32() { # <upstream|patched>
  cp build/dlls/opengl32/opengl32.so "$work/opengl32-$1.so"
  install_name_tool -id "$opengl32_install_name" "$work/opengl32-$1.so"
  for arch in i386 x86_64; do cp "build/dlls/opengl32/$arch-windows/opengl32.dll" "$work/opengl32-$1-$arch.dll"; done
}

configure
build
cp build/dlls/winemac.drv/winemac.so "$work/winemac-upstream.so"
install_name_tool -id "$install_name" "$work/winemac-upstream.so"
for arch in i386 x86_64; do cp "build/dlls/wined3d/$arch-windows/wined3d.dll" "$work/wined3d-upstream-$arch.dll"; done
cp build/dlls/wow64cpu/x86_64-windows/wow64cpu.dll "$work/wow64cpu-upstream.dll"
cp build/programs/wineboot/x86_64-windows/wineboot.exe "$work/wineboot-upstream.exe"
for arch in i386 x86_64; do cp "build/dlls/avrt/$arch-windows/avrt.dll" "$work/avrt-upstream-$arch.dll"; done
keep_opengl32 upstream
keep_wineserver upstream

for patch in "$here"/patches/*.patch; do
  patch -d src -p1 --forward <"$patch"
done
configure
build
cp build/dlls/winemac.drv/winemac.so "$work/winemac-patched.so"
install_name_tool -id "$install_name" "$work/winemac-patched.so"
cp build/dlls/win32u/win32u.so "$work/win32u-upstream.so"
for arch in i386 x86_64; do cp "build/dlls/wined3d/$arch-windows/wined3d.dll" "$work/wined3d-patched-$arch.dll"; done
cp build/dlls/wow64cpu/x86_64-windows/wow64cpu.dll "$work/wow64cpu-patched.dll"
cp build/programs/wineboot/x86_64-windows/wineboot.exe "$work/wineboot-patched.exe"
for arch in i386 x86_64; do cp "build/dlls/avrt/$arch-windows/avrt.dll" "$work/avrt-patched-$arch.dll"; done
keep_opengl32 patched
keep_wineserver patched

"$here/check-abi.sh" "$gcenx_winemac" "$gcenx_win32u" "$work/winemac-upstream.so" "$work/win32u-upstream.so" "$work/winemac-patched.so" | tee "$out/abi-check.txt"
for arch in i386 x86_64; do
  "$here/check-pe-exports.sh" "$work/llvm-mingw/bin/llvm-objdump" "$(find gcenx -path "*/$arch-windows/wined3d.dll")" \
    "$work/wined3d-upstream-$arch.dll" "$work/wined3d-patched-$arch.dll" | tee -a "$out/abi-check.txt"
done
"$here/check-pe-exports.sh" "$work/llvm-mingw/bin/llvm-objdump" "$(find gcenx -path '*/x86_64-windows/wow64cpu.dll')" \
  "$work/wow64cpu-upstream.dll" "$work/wow64cpu-patched.dll" | tee -a "$out/abi-check.txt"
"$here/check-unix-lib.sh" "$gcenx_opengl32" "$work/opengl32-upstream.so" "$work/opengl32-patched.so" | tee -a "$out/abi-check.txt"
for arch in i386 x86_64; do
  "$here/check-pe-exports.sh" "$work/llvm-mingw/bin/llvm-objdump" "$(find gcenx -path "*/$arch-windows/opengl32.dll")" \
    "$work/opengl32-upstream-$arch.dll" "$work/opengl32-patched-$arch.dll" | tee -a "$out/abi-check.txt"
done
"$here/check-pe-imports.sh" "$work/llvm-mingw/bin/llvm-objdump" "$(find gcenx -path '*/x86_64-windows/wineboot.exe')" \
  "$work/wineboot-upstream.exe" "$work/wineboot-patched.exe" | tee -a "$out/abi-check.txt"
for arch in i386 x86_64; do
  "$here/check-pe-exports.sh" "$work/llvm-mingw/bin/llvm-objdump" "$(find gcenx -path "*/$arch-windows/avrt.dll")" \
    "$work/avrt-upstream-$arch.dll" "$work/avrt-patched-$arch.dll" | tee -a "$out/abi-check.txt"
done
# Patch 0015 sets the policy of the server's own thread, which needs its port.
ALLOWED_NEW_IMPORTS='_mach_thread_self libSystem)' \
  "$here/check-unix-lib.sh" "$gcenx_wineserver" "$work/wineserver-upstream" "$work/wineserver-patched" program | tee -a "$out/abi-check.txt"

stage="$work/stage"
rm -rf "$stage" && mkdir -p "$stage/bin" "$stage/lib/wine/x86_64-unix" "$stage/lib/wine/i386-windows" "$stage/lib/wine/x86_64-windows" "$stage/share/doc/macron-wine"
cp "$work/winemac-patched.so" "$stage/lib/wine/x86_64-unix/winemac.so"
for arch in i386 x86_64; do cp "$work/wined3d-patched-$arch.dll" "$stage/lib/wine/$arch-windows/wined3d.dll"; done
cp "$work/wow64cpu-patched.dll" "$stage/lib/wine/x86_64-windows/wow64cpu.dll"
cp "$work/opengl32-patched.so" "$stage/lib/wine/x86_64-unix/opengl32.so"
for arch in i386 x86_64; do cp "$work/opengl32-patched-$arch.dll" "$stage/lib/wine/$arch-windows/opengl32.dll"; done
cp "$work/wineboot-patched.exe" "$stage/lib/wine/x86_64-windows/wineboot.exe"
for arch in i386 x86_64; do cp "$work/avrt-patched-$arch.dll" "$stage/lib/wine/$arch-windows/avrt.dll"; done
codesign --force --sign - "$stage/lib/wine/x86_64-unix/winemac.so"
codesign --force --sign - "$stage/lib/wine/x86_64-unix/opengl32.so"
cp "$work/wineserver-patched" "$stage/bin/wineserver"
chmod 755 "$stage/bin/wineserver"
codesign --force --sign - "$stage/bin/wineserver"
cp "$here"/patches/*.patch "$here/patches/README.md" src/COPYING.LIB "$out/abi-check.txt" "$stage/share/doc/macron-wine/"
tar -C "$stage" -cJf "$out/$ASSET_NAME" bin lib share
cp "$here"/patches/*.patch "$out/"
(cd "$out" && shasum -a 256 "$ASSET_NAME" | tee "$ASSET_NAME.sha256")
