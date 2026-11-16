#!/bin/bash
# measure-wined3d.sh <rounds>: two wined3d.dll builds (Wine patch 0008 before
# and after) on a Mac sandbox, alternating round by round. Each round, for
# each build, in an order that swaps every round:
#   - glprobe32.exe and glprobe64.exe --cost (tools/wow64-gl-probe): the time
#     of one OpenGL call, through `macron launch` (profile wow64-gl-probe);
#     they do not load wined3d, so they show the sandbox's state;
#   - probe32.exe and probe64.exe (scenarios/d3d9-fps-probe) in `up` mode with
#     3,900 sprites of 1 pixel, the window 1024x768 (profile d3d9-fps-probe);
# and once per round, without Wine, loadprobe (tools/clickteam-games), x86_64
# under Rosetta and arm64, at the same load.
# Every run uses MACRON_OPENGL=apple-software. Every Wine process is killed
# between runs. The builds are swapped into both runtime trees and every
# prefix (system32, syswow64), as ucn-run.sh does.
#
# Layout (prepare it first):
#   ~/gl/bundle: ci/build-bundle.sh's output (macron, the scenarios built),
#     plus glprobe32.exe, glprobe64.exe and loadprobe.m;
#   ~/gl/wd3d/{stock,patched}/{i386,x64}/wined3d.dll.
# Results: ~/gl/results-wd3d/results.txt, one line per run; logs beside it.
set -u
rounds=${1:-3}
G=$HOME/gl
M="$HOME/Library/Application Support/Macron"
MACRON=$G/bundle/bin/macron
RT=$(ls -d "$M"/runtimes/*/)
out=$G/results-wd3d
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

put_wined3d() { # stock|patched
  local src=$G/wd3d/$1 tree pfx
  for tree in wine wine-dxmt; do
    cp "$src/i386/wined3d.dll" "$RT/$tree/lib/wine/i386-windows/wined3d.dll"
    cp "$src/x64/wined3d.dll" "$RT/$tree/lib/wine/x86_64-windows/wined3d.dll"
  done
  for pfx in "$M"/prefixes/*/drive_c "$M"/prefixes/*/pfx/drive_c; do
    [ -d "$pfx/windows/system32" ] || continue
    cp "$src/x64/wined3d.dll" "$pfx/windows/system32/wined3d.dll"
    [ -d "$pfx/windows/syswow64" ] && cp "$src/i386/wined3d.dll" "$pfx/windows/syswow64/wined3d.dll"
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
  put_wined3d "$1"
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
put_wined3d patched
echo done >"$out/done"
