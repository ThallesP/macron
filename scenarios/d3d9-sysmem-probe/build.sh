#!/bin/sh
# The same probe as d3d9-fps-probe; this scenario runs it with --sysmem.
set -eu
exec sh "$(cd "$(dirname "$0")" && pwd)/../d3d9-fps-probe/build.sh" "$1"
