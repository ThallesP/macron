#!/bin/bash
# Downloads Valve's lsteamclient and steam_helper from Proton at the commit in
# release.env, checks them against its digests and against the licences in this
# repository, and applies Macron's patches. Nothing of Valve's is committed here.
# Usage: lsteamclient/fetch.sh <work-dir>
# Prints the folder holding the patched lsteamclient/ and steam_helper/.
set -euo pipefail

work=$1
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
. "$here/release.env"

digest() {
  (cd "$1" && find . -type f | LC_ALL=C sort | xargs "${sha256_cmd[@]}" | "${sha256_cmd[@]}" | cut -d ' ' -f 1)
}

if command -v shasum >/dev/null; then sha256_cmd=(shasum -a 256); else sha256_cmd=(sha256sum); fi

mkdir -p "$work"
work=$(cd "$work" && pwd)
tarball="$work/proton-$PROTON_COMMIT.tar.gz"
if [ ! -f "$tarball" ]; then
  curl -fsSL --retry 3 -o "$tarball.partial" "$PROTON_URL"
  mv "$tarball.partial" "$tarball"
fi

valve="$work/valve"
rm -rf "$valve" && mkdir -p "$valve"
top="Proton-$PROTON_COMMIT"
tar -xzf "$tarball" -C "$valve" --strip-components 1 "$top/lsteamclient" "$top/steam_helper" "$top/LICENSE.proton" >&2

check() {
  local name=$1 want=$2 got
  got=$(digest "$valve/$name")
  if [ "$got" != "$want" ]; then
    echo "$name from Proton $PROTON_COMMIT has digest $got, expected $want" >&2
    exit 1
  fi
  echo "$name: digest $got matches release.env ($(find "$valve/$name" -type f | wc -l | tr -d ' ') files)" >&2
}
check lsteamclient "$LSTEAMCLIENT_DIGEST"
check steam_helper "$STEAM_HELPER_DIGEST"
cmp -s "$valve/lsteamclient/LICENSE" "$repo/lsteamclient/LICENSE" || { echo "lsteamclient/LICENSE differs from Valve's" >&2; exit 1; }
cmp -s "$valve/LICENSE.proton" "$repo/steam_helper/LICENSE" || { echo "steam_helper/LICENSE differs from Proton's LICENSE.proton" >&2; exit 1; }

tree="$work/tree"
rm -rf "$tree" && mkdir -p "$tree"
cp -R "$valve/lsteamclient" "$valve/steam_helper" "$tree/"
for patch in "$repo"/lsteamclient/patches/*.patch; do
  patch -d "$tree/lsteamclient" -p1 --forward --quiet <"$patch"
done
for patch in "$repo"/steam_helper/patches/*.patch; do
  patch -d "$tree/steam_helper" -p1 --forward --quiet <"$patch"
done
echo "$tree"
