#!/bin/bash
# Game survey on a Mac sandbox (sbx): run one game from games.json under
# Wine 11.21 with DXMT v0.80 and screenshot the desktop.
#
#   mac.sh setup          install Wine and DXMT into ~/gs/wine
#   mac.sh run <game>     download, extract, configure, launch, screenshot
#
# Results go to ~/gs/out/<game>: screenshots, wine.log, result.txt.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
GS=$HOME/gs
WINE_URL=https://github.com/Gcenx/macOS_Wine_builds/releases/download/11.21/wine-devel-11.21-osx64.tar.xz
WINE_SHA=9909b486657e10ab66c7279371ae1f990345b596b3b1b3a0cf21f28af24fa0ce
DXMT_URL=https://github.com/3Shain/dxmt/releases/download/v0.80/dxmt-v0.80-builtin.tar.gz
DXMT_SHA=8f260e36b5739e68f3bad613381441385c4dc7b85b78ba8de653d5a6a264529d
DXVK_URL=https://github.com/Gcenx/DXVK-macOS/releases/download/v1.10.3-20230507/dxvk-macOS-async-v1.10.3-20230507.tar.gz
DXVK_SHA=f67d99d0a8eeedd7d406b283a3df9f939b5965acb00efcb33d0c6235c195a516
MESA_URL=https://github.com/pal1000/mesa-dist-win/releases/download/26.1.8/mesa3d-26.1.8-release-msvc.7z
MESA_SHA=4c6d32e653e0ff9ad07796e40c0bcfabf2764d849e3ce4f3b1590112c87e42f9

timeout_run() { perl -e 'alarm shift; exec @ARGV' "$@"; }

fetch() { # fetch <url> <sha256> -> prints path
  local url=$1 sha=$2 file
  file=$GS/dl/$(basename "${url%%\?*}")
  mkdir -p "$GS/dl"
  [ -f "$file" ] || curl -fsSL --retry 3 -o "$file" "$url"
  echo "$sha  $file" | shasum -a 256 -c - >/dev/null || { echo "SHA-256 mismatch: $url" >&2; exit 1; }
  echo "$file"
}

setup() {
  mkdir -p "$GS"
  local wine dxmt
  wine=$(fetch "$WINE_URL" "$WINE_SHA")
  dxmt=$(fetch "$DXMT_URL" "$DXMT_SHA")
  rm -rf "$GS/wine" "$GS/dxmt" && mkdir -p "$GS/wine" "$GS/dxmt"
  tar -xf "$wine" -C "$GS/wine"
  tar -xzf "$dxmt" -C "$GS/dxmt"
  local root
  root=$(ls -d "$GS"/wine/Wine*.app)/Contents/Resources/wine
  for arch in x86_64-windows x86_64-unix i386-windows; do
    cp -f "$GS"/dxmt/*/"$arch"/* "$root/lib/wine/$arch/"
  done
  ln -sfn "$root" "$GS/wineroot"
  "$GS/wineroot/bin/wine" --version
  command -v 7zz >/dev/null || HOMEBREW_NO_AUTO_UPDATE=1 brew install -q sevenzip
}

run() {
  local game=$1 spec
  spec=$(jq --arg g "$game" '.games[$g] as $s | if $s.same_as then .games[$s.same_as] + ($s | del(.same_as)) else $s end' "$HERE/games.json")
  [ "$spec" != null ] || { echo "unknown game $game" >&2; exit 2; }

  local out=$GS/out/$game
  rm -rf "$out" && mkdir -p "$out"
  log() { echo "$(date +%H:%M:%S) $*" | tee -a "$out/result.txt"; }

  export WINEPREFIX=$GS/prefix-$game
  export WINEDEBUG=${WINEDEBUG:-err+all,fixme-all}
  export WINEDLLOVERRIDES="mscoree,mshtml=;winemenubuilder.exe=d"
  export DXMT_LOG_LEVEL=info
  local wine=$GS/wineroot/bin/wine
  local root=$WINEPREFIX/drive_c/game
  local appdata=$WINEPREFIX/drive_c/users/$USER/AppData/Roaming

  if [ ! -d "$WINEPREFIX" ]; then
    local t=$(date +%s)
    timeout_run 420 "$wine" wineboot -u >"$out/wineboot.log" 2>&1 || true
    timeout_run 120 "$GS/wineroot/bin/wineserver" -w || true
    log "wineboot took $(( $(date +%s) - t ))s"
  fi

  local backend overrides="mscoree,mshtml=;winemenubuilder.exe=d"
  backend=$(jq -r .backend <<<"$spec")
  if [ "$backend" = dxvk ]; then
    # DXVK-macOS over MoltenVK, set up the way main's launcher does it.
    local dxvk sys32=$WINEPREFIX/drive_c/windows/system32
    dxvk=$(fetch "$DXVK_URL" "$DXVK_SHA")
    rm -rf "$GS/dxvk" && mkdir -p "$GS/dxvk" && tar -xzf "$dxvk" -C "$GS/dxvk" 2>/dev/null
    cp -f "$GS"/dxvk/*/x64/{d3d9,d3d11,d3d10core,dxgi}.dll "$sys32/"
    overrides="$overrides;d3d9,d3d11,d3d10core,dxgi=n"
  fi
  export WINEDLLOVERRIDES=$overrides
  local k v
  while IFS='=' read -r k v; do
    [ -n "$k" ] && export "$k=$v" && log "env $k=$v"
  done < <(jq -r '.mac_env // {} | to_entries[] | "\(.key)=\(.value)"' <<<"$spec")

  local t0=$(date +%s) n i
  n=$(jq '.downloads | length' <<<"$spec")
  for (( i = 0; i < n; i++ )); do
    local d url sha method into file
    d=$(jq ".downloads[$i]" <<<"$spec")
    url=$(jq -r .url <<<"$d"); sha=$(jq -r .sha256 <<<"$d")
    method=$(jq -r .extract <<<"$d"); into=$root/$(jq -r .into <<<"$d")
    file=$(fetch "$url" "$sha")
    log "downloaded $url"
    mkdir -p "$into"
    case $method in
      zip|7z) 7zz x -y "-o$into" "$file" >/dev/null ;;
      inno-silent)
        local setup=$file inner
        inner=$(jq -r '.inner // empty' <<<"$d")
        if [ -n "$inner" ]; then 7zz x -y "-o$GS/setup-$game" "$file" >/dev/null; setup=$GS/setup-$game/$inner; fi
        timeout_run 900 "$wine" "$setup" /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP- "/DIR=C:\\game" >>"$out/install.log" 2>&1 || log "installer failed"
        ;;
      nsis-silent)
        timeout_run 900 "$wine" "$file" /S "/D=C:\\game" >>"$out/install.log" 2>&1 || log "installer failed"
        timeout_run 120 "$GS/wineroot/bin/wineserver" -w || true
        ;;
      *) echo "unknown extract $method" >&2; exit 1 ;;
    esac
  done
  log "extracted in $(( $(date +%s) - t0 ))s"

  local exe cwd
  exe=$root/$(jq -r .exe <<<"$spec")
  [ -f "$exe" ] || { log "exe not found: $exe"; find "$root" -maxdepth 4 -iname '*.exe' | head -20 | tee -a "$out/result.txt"; exit 1; }
  cwd=$(jq -r '.cwd // empty' <<<"$spec")
  if [ -n "$cwd" ]; then cwd=$root/$cwd; else cwd=$(dirname "$exe"); fi

  # One-time steps after unpacking, moves, and configs derived from shipped files.
  local np step_exe step_cwd
  np=$(jq '.prelaunch // [] | length' <<<"$spec")
  for (( i = 0; i < np; i++ )); do
    step_exe=$root/$(jq -r ".prelaunch[$i].exe" <<<"$spec")
    step_cwd=$root/$(jq -r ".prelaunch[$i].cwd" <<<"$spec")
    local step_args=()
    while IFS= read -r a; do step_args+=("$a"); done < <(jq -r ".prelaunch[$i].args[]" <<<"$spec")
    local rc=0
    (cd "$step_cwd" && timeout_run 300 "$wine" "$step_exe" "${step_args[@]}") >>"$out/prelaunch.log" 2>&1 || rc=$?
    log "prelaunch $(basename "$step_exe") ${step_args[*]}: exit $rc"
  done
  local nm
  nm=$(jq '.moves // [] | length' <<<"$spec")
  for (( i = 0; i < nm; i++ )); do
    mv -f "$root/$(jq -r ".moves[$i].from" <<<"$spec")" "$root/$(jq -r ".moves[$i].to" <<<"$spec")" || log "move $i failed"
  done
  local nd from
  nd=$(jq '.derive // [] | length' <<<"$spec")
  for (( i = 0; i < nd; i++ )); do
    from=$(jq -r ".derive[$i].from" <<<"$spec"); from=${from//\{game\}/$root}
    to=$(jq -r ".derive[$i].to" <<<"$spec"); to=${to//\{game\}/$root}
    jq -r ".derive[$i].replace[] | @json" <<<"$spec" > "$GS/replace.json"
    python3 -I -c 'import json,sys
s=open(sys.argv[1],encoding="latin-1").read()
for line in open(sys.argv[3]): a,b=json.loads(line); s=s.replace(a,b)
open(sys.argv[2],"w",encoding="latin-1").write(s)' "$from" "$to" "$GS/replace.json"
    log "derived $to"
  done

  local nf to
  nf=$(jq '.files // [] | length' <<<"$spec")
  for (( i = 0; i < nf; i++ )); do
    to=$(jq -r ".files[$i].to" <<<"$spec")
    to=${to//\{game\}/$root}; to=${to//\{appdata\}/$appdata}; to=${to//\{documents\}/$WINEPREFIX/drive_c/users/$USER/Documents}
    mkdir -p "$(dirname "$to")"
    cp "$HERE/config/$(jq -r ".files[$i].from" <<<"$spec")" "$to"
    log "wrote $to"
  done

  if [ "$(jq -r .opengl <<<"$spec")" = true ]; then
    # Stock winemac only offers accelerated GL pixel formats and this GPU has
    # none, so OpenGL games get Mesa's llvmpipe opengl32.dll next to the exe,
    # the same renderer the Windows reference uses. It presents through GDI.
    local mesa
    mesa=$(fetch "$MESA_URL" "$MESA_SHA")
    7zz x -y "-o$GS/mesa" "$mesa" x64/opengl32.dll x64/libgallium_wgl.dll >/dev/null
    cp -f "$GS"/mesa/x64/*.dll "$(dirname "$exe")/"
    export WINEDLLOVERRIDES="$WINEDLLOVERRIDES;opengl32=n,b" GALLIUM_DRIVER=llvmpipe
    log "Mesa llvmpipe opengl32.dll placed next to the exe"
  fi
  log "WINEDLLOVERRIDES=$WINEDLLOVERRIDES"

  local args=()
  while IFS= read -r a; do args+=("$a"); done < <(jq -r '.args[]' <<<"$spec")

  screencapture -x "$out/00-before.png" || true
  log "launch: $exe ${args[*]-} (cwd $cwd, backend $(jq -r .backend <<<"$spec"))"
  local start=$(date +%s)
  (cd "$cwd" && exec "$wine" "$exe" ${args[@]+"${args[@]}"}) >"$out/wine.log" 2>&1 &
  local pid=$!

  for s in $(jq -r '.shots[]' <<<"$spec") 240; do
    local wait=$(( start + s - $(date +%s) ))
    (( wait > 0 )) && sleep "$wait"
    local name
    name=$(printf '%03ds.png' "$s")
    screencapture -x "$out/$name" || log "screencapture failed"
    if kill -0 "$pid" 2>/dev/null; then log "shot $name alive=yes"; else log "shot $name alive=no"; break; fi
  done
  if ! kill -0 "$pid" 2>/dev/null; then wait "$pid" && log "exited 0" || log "exited with code $?"; fi

  timeout_run 30 "$GS/wineroot/bin/wineserver" -k || true
  local l p
  while IFS= read -r l; do
    p=${l//\{game\}/$root}; p=${p//\{appdata\}/$appdata}; p=${p//\{documents\}/$WINEPREFIX/drive_c/users/$USER/Documents}
    if [ -f "$p" ]; then cp "$p" "$out/"; else for f in $p; do [ -f "$f" ] && cp "$f" "$out/" || true; done; fi
  done < <(jq -r '.logs[]?' <<<"$spec")
  grep -iE 'err:|dxmt|metal|failed|exception|error' "$out/wine.log" | sort | uniq -c | sort -rn | head -40 > "$out/wine-errors.txt" || true
  log done
}

case ${1-} in
  setup) setup ;;
  run) run "$2" ;;
  *) echo "usage: mac.sh setup | run <game>" >&2; exit 2 ;;
esac
