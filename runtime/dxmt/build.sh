#!/bin/bash
# Builds the DXMT modules Macron's patches change, from the DXMT release
# Macron's runtime downloads (release.env) with the patches in
# runtime/dxmt/patches, and packs them as an overlay for the DXMT tree:
#   lib/wine/x86_64-windows/{d3d11,winemetal,dxgi,nvngx}.dll
#   lib/wine/i386-windows/{d3d11,winemetal,dxgi}.dll
#   lib/wine/x86_64-unix/winemetal.so
# plus the patches, the patched source files and the licences (DXMT is LGPL
# 2.1+). They belong together: d3d11.dll calls winemetal.dll, which calls
# winemetal.so by number, and dxgi.dll carries the certificate nvngx.dll is
# signed with. DXMT's d3d10core.dll and nvapi64.dll stay as released.
#
# nvngx.dll is signed, because NVIDIA's NGX (what a game with DLSS links)
# loads it only when its signature chains to a root the machine trusts. The
# key is made here for this one build and deleted before the script ends;
# its certificate is compiled into dxgi.dll, which adds it to a prefix's Root
# store when the NVIDIA extension is on (patch 0005). So every build has its
# own certificate and nobody holds a key that a prefix trusts.
#
# Built as DXMT's own CI builds them: the Windows side with the llvm-mingw
# release its CI uses, then `winebuild --builtin` (without that mark Wine does
# not take a DLL as its builtin), then stripped; winemetal.so with Xcode's
# clang for x86_64 against LLVM 15.0.7 (DXMT's shader compiler links it
# statically), for macOS 15.0 and later as DXMT's release is.
#
# LLVM takes most of the time (about a quarter of an hour on a runner). It is
# built once into <work-dir>/llvm-darwin and reused while that folder is
# there; DXMT_LLVM=<dir> names another build of it.
# Runs on a GitHub macOS runner with Homebrew's cmake and ninja; installs
# Homebrew's osslsigncode when it is missing.
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
export MACOSX_DEPLOYMENT_TARGET=$MACOS_TARGET

fetch "$LLVM_MINGW_URL" "$LLVM_MINGW_SHA256" llvm-mingw.tar.xz
rm -rf llvm-mingw && mkdir llvm-mingw
tar -xJf llvm-mingw.tar.xz -C llvm-mingw --strip-components 1
# DXMT's configure step needs the Wine import libraries (winecrt0, ntdll,
# dbghelp) its winemetal.dll links against, and winemetal.so links Wine's
# winemac.so and ntdll.so; DXMT's CI takes them from this Wine build, and so
# does this script.
fetch "$DXMT_WINE_URL" "$DXMT_WINE_SHA256" dxmt-wine.tar.gz
rm -rf wine && mkdir wine
tar -xzf dxmt-wine.tar.gz -C wine
wine_tree=$work/wine

python3 -m venv tools
tools/bin/pip install --quiet "meson==$MESON_VERSION" "ninja==$NINJA_VERSION"
PATH="$work/tools/bin:$work/llvm-mingw/bin:$PATH"

llvm=${DXMT_LLVM:-$work/llvm-darwin}
if [ ! -f "$llvm/lib/libLLVMCore.a" ]; then
  rm -rf llvm-project llvm-darwin-build
  git clone --quiet --depth 1 --branch "$LLVM_TAG" "$LLVM_REPO" llvm-project
  [ "$(git -C llvm-project rev-parse HEAD)" = "$LLVM_COMMIT" ] || { echo "LLVM $LLVM_TAG is not $LLVM_COMMIT" >&2; exit 1; }
  # DXMT's CI job setup-llvm-darwin, with the macOS target added.
  cmake -B llvm-darwin-build -S llvm-project/llvm -G Ninja \
    -DCMAKE_INSTALL_PREFIX="$llvm" \
    -DCMAKE_OSX_ARCHITECTURES=x86_64 \
    -DCMAKE_OSX_DEPLOYMENT_TARGET="$MACOS_TARGET" \
    -DLLVM_HOST_TRIPLE=x86_64-apple-darwin \
    -DLLVM_ENABLE_ASSERTIONS=On \
    -DLLVM_ENABLE_ZSTD=Off \
    -DCMAKE_BUILD_TYPE=Release \
    -DLLVM_TARGETS_TO_BUILD="" \
    -DLLVM_BUILD_TOOLS=Off \
    -DBUG_REPORT_URL="https://github.com/3Shain/dxmt" \
    -DPACKAGE_VENDOR="DXMT" \
    -DLLVM_VERSION_PRINTER_SHOW_HOST_TARGET_INFO=Off \
    -DCMAKE_POLICY_VERSION_MINIMUM=3.5 > "$work/llvm-configure.log" 2>&1 || { tail -40 "$work/llvm-configure.log"; exit 1; }
  cmake --build llvm-darwin-build > "$work/llvm-build.log" 2>&1 || { tail -60 "$work/llvm-build.log"; exit 1; }
  cmake --install llvm-darwin-build > "$work/llvm-install.log" 2>&1
  cp llvm-project/llvm/LICENSE.TXT "$llvm/LICENSE.TXT"
  rm -rf llvm-project llvm-darwin-build
fi

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
  # Patch 0006's rule for the frame rate of a hot Mac, by itself, before anything is built with it
  c++ -std=c++20 -I dxmt/src/dxmt -o thermal_pacer "$here/tests/thermal_pacer.cpp"
  ./thermal_pacer
fi
# The 32-bit build makes no winemetal.so, so nothing in it links LLVM.
mkdir -p no-llvm/include no-llvm/lib

signed=no
if [ -f dxmt/src/dxgi/dxgi_ngx_certificate.h ]; then
  signed=yes
  command -v osslsigncode > /dev/null || brew install --quiet osslsigncode
  openssl=$(brew --prefix openssl@3)/bin/openssl
  rm -f ngx-key.pem ngx-certificate.pem
  "$openssl" req -x509 -newkey rsa:2048 -nodes -sha256 -days 10950 -subj "/CN=Macron NGX ($RELEASE_TAG)" \
    -addext "basicConstraints=critical,CA:FALSE" -addext "keyUsage=critical,digitalSignature" \
    -addext "extendedKeyUsage=codeSigning" -keyout ngx-key.pem -out ngx-certificate.pem 2> "$work/ngx-key.log" ||
    { cat "$work/ngx-key.log"; exit 1; }
  trap 'rm -f "$work/ngx-key.pem"' EXIT
  {
    echo "#pragma once"
    echo
    echo "// The certificate this build's nvngx.dll is signed with, in DER; written by Macron's runtime/dxmt/build.sh."
    echo "static const unsigned char macron_ngx_certificate[] = {"
    "$openssl" x509 -in ngx-certificate.pem -outform der | xxd -i
    echo "};"
  } > dxmt/src/dxgi/dxgi_ngx_certificate.h
fi

build() { # <cross file> <arch> <LLVM> <meson options> <modules> [<other targets>]
  local cross=$1 arch=$2 llvm_path=$3 options=$4 modules=$5 dir=build-$2 module
  shift 5
  rm -rf "dxmt/$dir"
  # shellcheck disable=SC2086
  (cd dxmt && meson setup --cross-file "$cross" -Dnative_llvm_path="$llvm_path" -Dwine_install_path="$wine_tree" \
    $options --buildtype release "$dir" > "$work/setup-$arch.log" 2>&1) || { tail -40 "$work/setup-$arch.log"; exit 1; }
  for module in $modules; do
    set -- "$@" "src/$module/$module.dll" "src/$module/$module.dll.postproc"
  done
  ninja -C "dxmt/$dir" "$@"
  mkdir -p "$out/overlay/lib/wine/$arch-windows"
  for module in $modules; do
    cp "dxmt/$dir/src/$module/$module.dll" "$out/overlay/lib/wine/$arch-windows/$module.dll"
    llvm-strip "$out/overlay/lib/wine/$arch-windows/$module.dll"
    grep -c 'Wine builtin DLL' "$out/overlay/lib/wine/$arch-windows/$module.dll" > /dev/null || { echo "$arch $module.dll has no Wine builtin mark" >&2; exit 1; }
  done
}

build build-win64.txt x86_64 "$llvm" -Denable_nvngx=true "d3d11 winemetal dxgi nvngx" src/winemetal/unix/winemetal.so
build build-win32.txt i386 "$work/no-llvm" "" "d3d11 winemetal dxgi"

# Signed last: stripping or marking a signed file would break its signature.
ngx=$out/overlay/lib/wine/x86_64-windows/nvngx.dll
if [ $signed = yes ]; then
  osslsigncode sign -certs ngx-certificate.pem -key ngx-key.pem -n "Macron NGX" -h sha256 -in "$ngx" -out "$ngx.signed" > "$work/ngx-sign.log" 2>&1 ||
    { cat "$work/ngx-sign.log"; exit 1; }
  mv "$ngx.signed" "$ngx"
  rm -f ngx-key.pem
  osslsigncode verify -CAfile ngx-certificate.pem -in "$ngx" > "$work/ngx-verify.log" 2>&1 ||
    { cat "$work/ngx-verify.log"; echo "nvngx.dll's signature does not verify against this build's certificate" >&2; exit 1; }
  grep -c 'Wine builtin DLL' "$ngx" > /dev/null || { echo "the signed nvngx.dll lost its Wine builtin mark" >&2; exit 1; }
fi

unix_lib=$out/overlay/lib/wine/x86_64-unix/winemetal.so
mkdir -p "$(dirname "$unix_lib")"
cp dxmt/build-x86_64/src/winemetal/unix/winemetal.so "$unix_lib"
strip -x "$unix_lib"
[ "$(lipo -archs "$unix_lib")" = x86_64 ] || { echo "winemetal.so is not x86_64 only: $(lipo -archs "$unix_lib")" >&2; exit 1; }
minos=$(otool -l "$unix_lib" | awk '/LC_BUILD_VERSION/ {found = 1} found && $1 == "minos" {print $2; exit}')
[ "$minos" = "$MACOS_TARGET" ] || { echo "winemetal.so is built for macOS $minos, not $MACOS_TARGET" >&2; exit 1; }
# The Windows side calls the Unix side by position in this table, so both
# sides must come from the same source. (grep reads to the end: with pipefail
# a grep that stops at the first match fails the pipe.)
nm -g "$unix_lib" | grep -c ' ___wine_unix_call_funcs$' > /dev/null || { echo "winemetal.so has no __wine_unix_call_funcs" >&2; exit 1; }
if otool -L "$unix_lib" | grep -v -E '^\S|@rpath/(winemetal|winemac|ntdll)\.so|/System/Library/|/usr/lib/'; then
  echo "winemetal.so depends on a library outside macOS and Wine" >&2
  exit 1
fi

share=$out/overlay/share/macron/dxmt
mkdir -p "$share"
cp "$here"/patches/*.patch dxmt/LICENSE "$share/"
cp "$llvm/LICENSE.TXT" "$share/LLVM-LICENSE.TXT"
[ $signed = no ] || cp ngx-certificate.pem "$share/ngx-certificate.pem"
# The files the patches change, as built.
(cd dxmt && git add -A -N . && git diff --name-only HEAD -- src dxmt.conf | tar -czf "$share/patched-source.tar.gz" -T -)
{
  echo "DXMT $DXMT_TAG ($DXMT_COMMIT) with Macron's patches; $RELEASE_TAG"
  echo "Windows side built with $(x86_64-w64-mingw32-gcc --version | head -1), meson $MESON_VERSION"
  echo "winemetal.so built with $(cc --version | head -1) for macOS $MACOS_TARGET, with LLVM $LLVM_TAG ($LLVM_COMMIT) linked statically"
  [ $signed = no ] || echo "nvngx.dll signed with this build's own certificate: $("$openssl" x509 -in ngx-certificate.pem -noout -fingerprint -sha256)"
} > "$share/VERSION"
tar -cJf "$out/$ASSET_NAME" -C "$out/overlay" .
(cd "$out" && shasum -a 256 "$ASSET_NAME" | tee "$ASSET_NAME.sha256")
