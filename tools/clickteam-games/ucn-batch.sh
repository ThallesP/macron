#!/bin/bash
# ucn-batch.sh <config>... : the configurations of docs/gate1-ultimate-custom-night.md,
# run one after another through ucn-run.sh (3 starts each unless UCN_STARTS).
#   a        macron.5 as shipped, MACRON_OPENGL=apple-software
#   b        a with Gcenx's stock wined3d.dll (no 0003, no 0004)
#   c        a with WINE_D3D_CONFIG=csmt=0x1 (0004's command stream thread back on)
#   d        a with Gcenx's stock wow64cpu.dll (no 0006)
#   e        no MACRON_OPENGL, prefix without AllowSoftwareRendering: wined3d
#            gets no adapter and the game uses Fusion's own software display;
#            +bitblt traced so its frames can be counted
#   e-quiet  e without the trace, for the CPU it uses untraced
#   f-llvmpipe  MACRON_OPENGL=llvmpipe: Mesa's OpenGL inside the 32-bit process,
#            so no WoW64 thunk per OpenGL call, rasterised on every core
#   trace-m5, trace-stock  one start each with +opengl: OpenGL calls per frame
#   sample-m5  one start of a with macOS's `sample` on the game's process
set -u
here=$(cd "$(dirname "$0")" && pwd)
n=${UCN_STARTS:-3}
for c in "$@"; do
  case $c in
    a) MACRON_OPENGL=apple-software WINEDEBUG=-all,+fps UCN_BASE=apple "$here/ucn-run.sh" a "$n" ;;
    b) MACRON_OPENGL=apple-software WINEDEBUG=-all,+fps UCN_BASE=apple UCN_WINED3D=stock "$here/ucn-run.sh" b "$n" ;;
    c) MACRON_OPENGL=apple-software WINEDEBUG=-all,+fps UCN_BASE=apple WINE_D3D_CONFIG=csmt=0x1 "$here/ucn-run.sh" c "$n" ;;
    d) MACRON_OPENGL=apple-software WINEDEBUG=-all,+fps UCN_BASE=apple UCN_WOW64CPU=stock "$here/ucn-run.sh" d "$n" ;;
    e) WINEDEBUG=-all,+bitblt UCN_BASE=default "$here/ucn-run.sh" e "$n" ;;
    e-quiet) WINEDEBUG=-all UCN_BASE=default "$here/ucn-run.sh" e-quiet "$n" ;;
    f-llvmpipe) MACRON_OPENGL=llvmpipe WINEDEBUG=-all,+fps UCN_BASE=default "$here/ucn-run.sh" f-llvmpipe "$n" ;;
    trace-m5) UCN_MEASURE=8 MACRON_OPENGL=apple-software WINEDEBUG=-all,+opengl,+fps UCN_BASE=apple "$here/ucn-run.sh" trace-m5 1 ;;
    trace-stock) UCN_MEASURE=8 MACRON_OPENGL=apple-software WINEDEBUG=-all,+opengl,+fps UCN_BASE=apple UCN_WINED3D=stock "$here/ucn-run.sh" trace-stock 1 ;;
    sample-m5) UCN_SAMPLE=1 MACRON_OPENGL=apple-software WINEDEBUG=-all,+fps UCN_BASE=apple "$here/ucn-run.sh" sample-m5 1 ;;
    *) echo "unknown configuration $c" >&2 ;;
  esac
done
echo all-done >"$HOME/ucn/runs/batch-done"
