#!/bin/bash
# Builds Macron's installer on a Mac: macron as a universal binary (arm64 and
# x86_64), a component package that puts it and macron-uninstall in
# /usr/local/bin, a product archive around it, and a DMG holding that archive
# and the install notes. Usage: ci/package/build.sh <version x.y.z> <out-dir>
set -euo pipefail
version=$1
out=$2
if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must look like 1.2.3, got '$version'" >&2
  exit 2
fi

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
[ -d "$HOME/.local/go/bin" ] && PATH=$PATH:$HOME/.local/go/bin

payload="$work/payload/usr/local/bin"
mkdir -p "$payload" "$work/dmg" "$out"
for arch in arm64 amd64; do
  (cd "$root" && GOOS=darwin GOARCH=$arch CGO_ENABLED=0 go build -trimpath -o "$work/macron-$arch" ./cmd/macron)
done
lipo -create -output "$payload/macron" "$work/macron-arm64" "$work/macron-amd64"
lipo "$payload/macron" -verify_arch arm64 x86_64
install -m 755 "$here/macron-uninstall" "$payload/macron-uninstall"
chmod 755 "$payload/macron" "$here/scripts/postinstall"

pkgbuild --root "$work/payload" --install-location / --scripts "$here/scripts" \
  --identifier io.github.thallesp.macron --version "$version" "$work/macron.pkg"
sed "s/@VERSION@/$version/" "$here/distribution.xml" > "$work/distribution.xml"
productbuild --distribution "$work/distribution.xml" --resources "$here/resources" \
  --package-path "$work" "$work/dmg/Macron.pkg"
cp "$root/docs/install.md" "$work/dmg/Read me.txt"

dmg="$out/Macron-$version.dmg"
hdiutil create -volname "Macron $version" -srcfolder "$work/dmg" -format UDZO -ov "$dmg"
(cd "$out" && shasum -a 256 "$(basename "$dmg")" > "$(basename "$dmg").sha256")
cat "$dmg.sha256"
