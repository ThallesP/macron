#!/bin/bash
# Builds the Steam Play switch library on a Mac, for both architectures: Steam
# for Mac is universal, and dyld stops a program whose inserted library lacks
# its architecture. Usage: steamplay/build.sh <out.dylib>
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
out=$1
clang -O2 -Wall -Wextra -Werror -dynamiclib -arch arm64 -arch x86_64 -mmacosx-version-min=11.0 \
  -install_name @loader_path/macron-steamplay.dylib -o "$out" "$here/steamplay.c"
lipo "$out" -verify_arch arm64 x86_64
