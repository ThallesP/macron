#!/bin/bash
# Builds Wine's own conformance tests for the sound modules from the tree
# runtime/wine/build-overlay.sh leaves behind, so that a patched driver can
# be held against what Windows does. Runs on an Apple silicon Mac (a
# sandbox) after build-overlay.sh.
#
#   tools/wine-tests/build.sh <work-dir of build-overlay.sh>
#
# The tests land in <work-dir>/build-tests/dlls/<module>/tests/x86_64-windows/.
# build-overlay.sh configures without the test programs; this configures a
# second tree with them, 64-bit only, and builds three: mmdevapi, winmm and
# dsound. About eight minutes on a sandbox.
set -euo pipefail

work=$(cd "$1" && pwd)
cd "$work"
PATH="$(brew --prefix bison)/bin:$PATH"
rm -rf build-tests && mkdir build-tests && cd build-tests
MACOSX_DEPLOYMENT_TARGET=10.15 arch -x86_64 ../src/configure -C \
  --enable-archs=x86_64 --with-mingw="$work/llvm-mingw/bin/clang" \
  --disable-winebth_sys \
  --without-alsa --without-capi --with-coreaudio --without-cups --without-dbus \
  --without-ffmpeg --without-fontconfig --without-freetype --without-gettext \
  --without-gettextpo --without-gphoto --without-gnutls --without-gssapi \
  --without-gstreamer --without-inotify --without-krb5 --without-netapi \
  --without-opencl --without-opengl --without-oss --without-pcap --without-pcsclite \
  --with-pthread --without-pulse --without-sane --without-sdl --without-udev \
  --without-usb --without-v4l2 --without-vulkan --without-wayland --without-x \
  CFLAGS=-O2 >configure.log 2>&1 || { tail -20 configure.log; exit 1; }
MACOSX_DEPLOYMENT_TARGET=10.15 arch -x86_64 make -j"$(sysctl -n hw.ncpu)" \
  dlls/mmdevapi/tests/x86_64-windows/mmdevapi_test.exe \
  dlls/winmm/tests/x86_64-windows/winmm_test.exe \
  dlls/dsound/tests/x86_64-windows/dsound_test.exe >make.log 2>&1 || { tail -20 make.log; exit 1; }
ls -l dlls/mmdevapi/tests/x86_64-windows/mmdevapi_test.exe dlls/winmm/tests/x86_64-windows/winmm_test.exe \
  dlls/dsound/tests/x86_64-windows/dsound_test.exe
