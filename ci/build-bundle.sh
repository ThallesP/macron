#!/bin/sh
# Builds what a scenario check needs on the target machine: macron and
# macron-check for the target OS, every scenario, and the programs the
# scenarios build themselves. Usage: ci/build-bundle.sh <out-dir> [darwin|windows]
set -eu
out=$1
target=${2:-darwin}
root=$(cd "$(dirname "$0")/.." && pwd)
[ -d "$HOME/.local/go/bin" ] && PATH=$PATH:$HOME/.local/go/bin

rm -rf "$out"
mkdir -p "$out/bin"
cd "$root"
case $target in
  darwin) GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o "$out/bin/" ./cmd/macron ./cmd/macron-check ;;
  windows) GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$out/bin/" ./cmd/macron-check ;;
  *) echo "unknown target $target" >&2; exit 2 ;;
esac
cp -R scenarios "$out/"
for script in "$out"/scenarios/*/build.sh; do
  [ -e "$script" ] || continue
  sh "$script" "$(dirname "$script")"
done
