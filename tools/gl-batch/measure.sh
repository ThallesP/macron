#!/bin/bash
# measure.sh <rounds>: OpenGL call batching (Wine patch 0007) before and
# after, on a Mac sandbox, alternating the two opengl32 builds round by round.
# Each round, for each build, in an order that swaps every round:
#   - glprobe32.exe and glprobe64.exe --cost (tools/wow64-gl-probe): the time
#     of one OpenGL call, through `macron launch` (profile wow64-gl-probe);
#   - probe32.exe and probe64.exe (scenarios/d3d9-fps-probe) in `up` mode with
#     3,900 sprites of 1 pixel, the window 1024x768 (profile d3d9-fps-probe);
# and once per round, without Wine, loadprobe (tools/clickteam-games), x86_64
# under Rosetta and arm64, at the same load.
# Every run uses MACRON_OPENGL=apple-software. Every Wine process is killed
# between runs. The builds are swapped into both runtime trees and every
# prefix (system32, syswow64), the way ucn-run.sh swaps wined3d.dll.
#
# Layout (prepare it first):
#   ~/gl/bundle: ci/build-bundle.sh's output (macron, the scenarios built),
#     plus glprobe32.exe, glprobe64.exe and loadprobe.m;
#   ~/gl/opengl32/{stock,patched}/{opengl32.so,i386/opengl32.dll,x64/opengl32.dll}.
# Results: ~/gl/results/results.txt, one line per run; logs beside it.
set -u
rounds=${1:-3}
G=$HOME/gl
M="$HOME/Library/Application Support/Macron"
MACRON=$G/bundle/bin/macron
RT=$(ls -d "$M"/runtimes/*/)
out=$G/results
res=$out/results.txt
mkdir -p "$out"
export MACRON_OPENGL=apple-software WINEDEBUG=-all

kill_wine() {
  "$MACRON" kill --game d3d9-fps-probe >/dev/null 2>&1
  "$MACRON" kill --game wow64-gl-probe >/dev/null 2>&1
  pkill -9 -f 'Macron/runtimes/' 2>/dev/null
  pkill -9 -f '^[A-Za-z]:\\' 2>/dev/null
  sleep 2
}

put_gl() { # stock|patched
  local src=$G/opengl32/$1 tree pfx
  for tree in wine wine-dxmt; do
    cp "$src/opengl32.so" "$RT/$tree/lib/wine/x86_64-unix/opengl32.so"
    cp "$src/i386/opengl32.dll" "$RT/$tree/lib/wine/i386-windows/opengl32.dll"
    cp "$src/x64/opengl32.dll" "$RT/$tree/lib/wine/x86_64-windows/opengl32.dll"
  done
  for pfx in "$M"/prefixes/*/drive_c "$M"/prefixes/*/pfx/drive_c; do
    [ -d "$pfx/windows/system32" ] || continue
    cp "$src/x64/opengl32.dll" "$pfx/windows/system32/opengl32.dll"
    [ -d "$pfx/windows/syswow64" ] && cp "$src/i386/opengl32.dll" "$pfx/windows/syswow64/opengl32.dll"
  done
}

run() { # <label> <timeout> <game> <exe> [args...]
  local label=$1 limit=$2 game=$3 exe=$4 log
  shift 4
  log="$out/$label.$(date +%s).log"
  kill_wine
  perl -e 'alarm shift; exec @ARGV' "$limit" "$MACRON" launch --game "$game" --cwd "$(dirname "$exe")" -- "$exe" "$@" >"$log" 2>&1
  local code=$?
  { grep -a '^RESULT \(cost\|fps\)' "$log" | sed "s|^|$label |"; } >>"$res" || true
  grep -aq '^RESULT \(cost\|fps\)' "$log" || echo "$label no result (exit $code): $(tail -2 "$log" | tr '\n' ' ')" >>"$res"
}

wine_set() { # <build>
  put_gl "$1"
  for bits in 32 64; do
    run "$1 glprobe$bits" 300 wow64-gl-probe "$G/bundle/glprobe$bits.exe" --rounds 1 --gets 1 --cost 20000
  done
  for bits in 32 64; do
    run "$1 probe$bits" 300 d3d9-fps-probe "$G/bundle/scenarios/d3d9-fps-probe/probe$bits.exe" \
      up --sprites 3900 --sprite-size 1 --window 1024x768 --warmup 5 --seconds 10
  done
}

[ -x "$G/loadprobe-x86_64" ] || clang -O2 -fobjc-arc -arch x86_64 -framework Cocoa -framework OpenGL -framework Metal \
  -framework QuartzCore -o "$G/loadprobe-x86_64" "$G/bundle/loadprobe.m"
[ -x "$G/loadprobe-arm64" ] || clang -O2 -fobjc-arc -arch arm64 -framework Cocoa -framework OpenGL -framework Metal \
  -framework QuartzCore -o "$G/loadprobe-arm64" "$G/bundle/loadprobe.m"

for round in $(seq 1 "$rounds"); do
  echo "# round $round $(date -u +%H:%M:%S)" >>"$res"
  if [ $((round % 2)) = 1 ]; then order="stock patched"; else order="patched stock"; fi
  for build in $order; do
    wine_set "$build"
  done
  kill_wine
  for arch in x86_64 arm64; do
    arch -$arch "$G/loadprobe-$arch" api=gl-sw draws=3900 quad=1x1 uniforms=1 warm=3 seconds=10 2>&1 |
      grep '^RESULT' | sed "s|^|loadprobe-$arch |" >>"$res"
  done
done
put_gl patched
echo done >"$out/done"
