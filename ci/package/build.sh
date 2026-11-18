#!/bin/bash
# Builds Macron's installer on a Mac: macron as a universal binary (arm64 and
# x86_64), the Steam Play switch library (steamplay/), a component package that
# puts macron and macron-uninstall in /usr/local/bin and the library in
# /usr/local/lib/macron, a product archive around it, and a DMG holding that
# archive and the install notes. The programs macron diagnose times calls with
# (tools/wow64-cost) go to /usr/local/lib/macron/diagnose, the Windows ones
# built with the llvm-mingw that runtime/wine/release.env pins.
# Usage: ci/package/build.sh <version x.y.z or x.y.z-rcN> <out-dir>
set -euo pipefail
umask 022
version=$1
out=$2
if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)?$ ]]; then
  echo "version must look like 1.2.3 or 1.2.3-rc1, got '$version'" >&2
  exit 2
fi
package_version=${version%%-*}

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
[ -d "$HOME/.local/go/bin" ] && PATH=$PATH:$HOME/.local/go/bin

payload="$work/payload/usr/local/bin"
library="$work/payload/usr/local/lib/macron"
mkdir -p "$payload" "$library" "$work/dmg" "$out"
for arch in arm64 amd64; do
  (cd "$root" && GOOS=darwin GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$version" -o "$work/macron-$arch" ./cmd/macron)
done
lipo -create -output "$payload/macron" "$work/macron-arm64" "$work/macron-amd64"
lipo "$payload/macron" -verify_arch arm64 x86_64
"$root/steamplay/build.sh" "$library/macron-steamplay.dylib"
chmod 644 "$library/macron-steamplay.dylib"
install -m 755 "$here/macron-uninstall" "$payload/macron-uninstall"

diagnose="$library/diagnose"
mkdir -p "$diagnose"
clang -O2 -arch arm64 -arch x86_64 -o "$diagnose/transition-cost" "$root/tools/wow64-cost/native.c"
. "$root/runtime/wine/release.env"
curl -fsSL --retry 3 -o "$work/llvm-mingw.tar.xz" "$LLVM_MINGW_URL"
echo "$LLVM_MINGW_SHA256  $work/llvm-mingw.tar.xz" | shasum -a 256 -c
tar -xJf "$work/llvm-mingw.tar.xz" -C "$work"
mingw=$(ls -d "$work"/llvm-mingw-*/bin)
"$mingw/i686-w64-mingw32-clang" -O2 -msse2 -mfpmath=sse -mconsole "$root/tools/wow64-cost/cost.c" -o "$diagnose/cost32.exe"
"$mingw/x86_64-w64-mingw32-clang" -O2 -mconsole "$root/tools/wow64-cost/cost.c" -o "$diagnose/cost64.exe"
chmod 755 "$diagnose"/*
mkdir -p "$work/scripts"
sed "s/@VERSION@/$version/" "$here/scripts/postinstall" > "$work/scripts/postinstall"
chmod 755 "$payload/macron" "$work/scripts/postinstall"

pkgbuild --root "$work/payload" --install-location / --scripts "$work/scripts" \
  --identifier io.github.thallesp.macron --version "$package_version" "$work/macron.pkg"
sed "s/@VERSION@/$package_version/" "$here/distribution.xml" > "$work/distribution.xml"
productbuild --distribution "$work/distribution.xml" --resources "$here/resources" \
  --package-path "$work" "$work/dmg/Macron.pkg"
install -m 644 "$root/docs/install.md" "$work/dmg/Read me.txt"

dmg="$out/Macron-$version.dmg"
hdiutil create -volname "Macron $version" -srcfolder "$work/dmg" -format UDZO -ov "$dmg"
(cd "$out" && shasum -a 256 "$(basename "$dmg")" > "$(basename "$dmg").sha256")
cat "$dmg.sha256"
