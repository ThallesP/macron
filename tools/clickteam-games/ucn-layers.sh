#!/bin/bash
# ucn-layers.sh: the load of Ultimate Custom Night's frames (gltrace: 4 draws
# on the title, about 2,100 in the menu, about 3,900 in a night, each a small
# textured alpha-blended quad with its own vertex upload and uniforms) at
# 1024x768, one layer at a time, on the same sandbox:
#   1. loadprobe, native arm64: Apple's software OpenGL renderer, and Metal
#   2. loadprobe, x86_64 under Rosetta: the same, translated (a Wine process
#      loads Apple's OpenGL as x86_64 code)
#   3. the Direct3D 9 probe (scenarios/d3d9-fps-probe) in Wine through
#      wined3d, 64-bit (no WoW64) and 32-bit (WoW64 thunks), as the game runs
#      (MACRON_OPENGL=apple-software, macron.5)
# Results: ~/ucn/layers/results.txt, one RESULT line per run.
set -u
U=$HOME/ucn
out=$U/layers
mkdir -p "$out"
res=$out/results.txt
pkill -9 -f 'wineserver|winedevice|Custom Night.exe|Macron/runtimes' 2>/dev/null
sleep 2

probe() { # <arch> <args>...
  local arch=$1; shift
  echo "# $arch $*" >>"$res"
  arch -"$arch" "$U/loadprobe" warm=3 seconds=10 "$@" 2>&1 | grep '^RESULT' >>"$res"
}
for arch in arm64 x86_64; do
  for api in gl-sw metal; do
    probe "$arch" api=$api draws=4 quad=full textures=4 uniforms=1
    probe "$arch" api=$api draws=2100 quad=1x768 textures=1 uniforms=1
    probe "$arch" api=$api draws=3900 quad=1x768 textures=1 uniforms=1
    probe "$arch" api=$api draws=3900 quad=1x1 textures=1 uniforms=1
    probe "$arch" api=$api draws=3900 quad=64x64 textures=1 uniforms=1
  done
  # How the vertices reach the buffer (OpenGL only): wined3d's ring, against
  # rewriting one small buffer, against orphaning it.
  probe "$arch" api=gl-sw vb=sub0 draws=3900 quad=1x768 textures=1 uniforms=1
  probe "$arch" api=gl-sw vb=orphan draws=3900 quad=1x768 textures=1 uniforms=1
done

# The Direct3D 9 probe through macron, its own prefix (profile d3d9-fps-probe).
cd "$U"
for exe in probe64.exe probe32.exe; do
  for n in 4 2100 3900; do
    echo "# wine $exe up --sprites $n --window 1024x768" >>"$res"
    MACRON_OPENGL=apple-software WINEDEBUG=-all perl -e 'alarm shift; exec @ARGV' 300 "$HOME/work/bin/macron" launch --game d3d9-fps-probe --cwd "$U" -- \
      "$U/$exe" up --sprites "$n" --window 1024x768 --warmup 5 --seconds 10 >"$out/$exe-$n.log" 2>&1
    grep '^RESULT fps' "$out/$exe-$n.log" >>"$res" || echo "no result: $(tail -2 "$out/$exe-$n.log" | tr '\n' ' ')" >>"$res"
    "$HOME/work/bin/macron" kill --game d3d9-fps-probe >/dev/null 2>&1
    pkill -9 -f 'wineserver|winedevice|Macron/runtimes' 2>/dev/null
    sleep 2
  done
done
echo done >"$out/done"
