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

  # The runner's paravirtual GPU has no accelerated OpenGL renderer, only
  # "Apple Software Renderer", which winemac ignores unless this is set.
  if [ "${MAC_SOFTWARE_GL:-1}" = 1 ]; then
    "$wine" reg add 'HKCU\Software\Wine\Mac Driver' /v AllowSoftwareRendering /t REG_SZ /d y /f >/dev/null 2>&1
    log "AllowSoftwareRendering=y"
  fi
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

  local nf to
  nf=$(jq '.files // [] | length' <<<"$spec")
  for (( i = 0; i < nf; i++ )); do
    to=$(jq -r ".files[$i].to" <<<"$spec")
    to=${to//\{game\}/$root}; to=${to//\{appdata\}/$appdata}
    mkdir -p "$(dirname "$to")"
    cp "$HERE/config/$(jq -r ".files[$i].from" <<<"$spec")" "$to"
    log "wrote $to"
  done

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
    p=${l//\{game\}/$root}; p=${p//\{appdata\}/$appdata}
    for f in $p; do [ -f "$f" ] && cp "$f" "$out/" || true; done
  done < <(jq -r '.logs[]?' <<<"$spec")
  grep -iE 'err:|dxmt|metal|failed|exception|error' "$out/wine.log" | sort | uniq -c | sort -rn | head -40 > "$out/wine-errors.txt" || true
  log done
}

case ${1-} in
  setup) setup ;;
  run) run "$2" ;;
  *) echo "usage: mac.sh setup | run <game>" >&2; exit 2 ;;
esac
