#!/bin/bash
# ucn-run.sh <label> <starts>: starts Ultimate Custom Night the way Steam's
# Play does (SteamAppId=871720, STEAM_COMPAT_DATA_PATH, `macron
# waitforexitandrun` from the game's folder) and measures three places in one
# start: the title screen, the character-select menu (Enter) and a night
# (the menu's GO! button). Each start gets a fresh APFS clone of a prepared
# prefix (ucn-setup.sh) and every Wine process is killed before and after.
#
# Per place: UCN_WARM seconds to settle (default 15; the night, which loads
# first, UCN_NIGHT_WARM, default 25), then UCN_MEASURE seconds
# measured (default 20): wined3d's frame counter lines (WINEDEBUG=+fps) or
# any other trace lines, each timestamped as it arrives, the CPU time of every
# process over the window (ps), and a screenshot of the game's window.
#
# Configuration, from the environment:
#   UCN_WINED3D=m5|stock   wined3d.dll (i386 and x86_64) put into the DXMT
#                          tree and the prefix: macron.5's or Gcenx's
#   UCN_WOW64CPU=m5|stock  wow64cpu.dll, the same way
#   UCN_BASE=apple|default the prefix to clone (with or without
#                          AllowSoftwareRendering)
#   WINEDEBUG, MACRON_OPENGL, WINE_D3D_CONFIG ... pass through to macron.
# Results: ~/ucn/runs/<label>/<n>.{log,phases,ps-*,png} and results.jsonl
# (ucn-summary.py reads them). macron.5's DLLs are put back at the end.
set -u
label=$1 starts=$2
U=$HOME/ucn
warm=${UCN_WARM:-15} meas=${UCN_MEASURE:-20}
wined3d=${UCN_WINED3D:-m5} wow64cpu=${UCN_WOW64CPU:-m5} base=${UCN_BASE:-apple}
game="$HOME/Library/Application Support/Steam/steamapps/common/Ultimate Custom Night"
M="$HOME/Library/Application Support/Macron"
R=$(ls -d "$M"/runtimes/*/)wine-dxmt/lib/wine
drive=$HOME/work/bin/drive
out=$U/runs/$label
mkdir -p "$out"

kill_wine() {
  pkill -INT -f 'macron waitforexitandrun' 2>/dev/null
  sleep 3
  pkill -9 -f 'macron waitforexitandrun' 2>/dev/null
  pkill -9 -f 'wineserver|winedevice|Custom Night.exe|Macron/runtimes|^[A-Za-z]:\\' 2>/dev/null
  sleep 2
}

put_dlls() { # <wined3d> <wow64cpu> [prefix]
  cp "$U/dll/$1/i386/wined3d.dll" "$R/i386-windows/wined3d.dll"
  cp "$U/dll/$1/x64/wined3d.dll" "$R/x86_64-windows/wined3d.dll"
  cp "$U/dll/$2/x64/wow64cpu.dll" "$R/x86_64-windows/wow64cpu.dll"
  if [ -n "${3:-}" ]; then
    cp "$U/dll/$1/i386/wined3d.dll" "$3/drive_c/windows/syswow64/wined3d.dll"
    cp "$U/dll/$1/x64/wined3d.dll" "$3/drive_c/windows/system32/wined3d.dll"
    cp "$U/dll/$2/x64/wow64cpu.dll" "$3/drive_c/windows/system32/wow64cpu.dll"
  fi
}
trap 'put_dlls m5 m5' EXIT

game_window() { "$drive" windows | awk -F'\t' '$4 == "wine" && $5 == "Ultimate Custom Night" { print; exit }'; }
mark() { echo "$1 $(python3 -I -c 'import time;print(time.time())')" >>"$out/$run.phases"; }
psnap() { ps -A -o pid=,time=,args= >"$out/$run.ps-$1"; }

# advance <drive command>... -- shown|gone <text>: activate Wine, send the
# commands, and wait up to 12 s for <text> to appear on (or leave) the game's
# window; up to 5 tries.
advance() {
  local cmds=() want text try i
  while [ "$1" != -- ]; do cmds+=("$1"); shift; done
  want=$2 text=$3
  for try in 1 2 3 4 5; do
    "$drive" activate wine >/dev/null 2>&1; sleep 1
    for c in "${cmds[@]}"; do "$drive" $c >/dev/null 2>&1; sleep 1; done
    for i in 1 2 3 4 5 6; do
      sleep 2
      if "$drive" find wine "Ultimate Custom Night" "$text" >/dev/null 2>&1; then
        [ "$want" = shown ] && { echo "$try" >>"$out/$run.tries"; return 0; }
      else
        [ "$want" = gone ] && { echo "$try" >>"$out/$run.tries"; return 0; }
      fi
    done
  done
  return 1
}

# measure <place>: settle, then the measured window with CPU snapshots and a screenshot.
measure() { # <place> [settle seconds]
  sleep "${2:-$warm}"
  "$drive" shot wine "Ultimate Custom Night" "$out/$run-$1-start.png" >/dev/null 2>&1
  psnap "$1-0"; mark "$1-start"
  if [ -n "${UCN_SAMPLE:-}" ]; then
    # macOS's sampler on the game's process: where its threads spend the window.
    gpid=$(ps -A -o pid=,args= | awk '/^ *[0-9]+ [A-Za-z]:\\.*Custom Night\.exe/ { print $1; exit }')
    [ -n "$gpid" ] && sample "$gpid" $((meas - 2)) 5 -mayDie -file "$out/$run-$1.sample" >/dev/null 2>&1 &
  fi
  sleep "$meas"
  mark "$1-end"; psnap "$1-1"
  "$drive" shot wine "Ultimate Custom Night" "$out/$run-$1.png" >/dev/null 2>&1
}

for run in $(seq 1 "$starts"); do
  kill_wine
  compat=$U/compat/$label-$run
  rm -rf "$compat"; mkdir -p "$compat"
  cp -cR "$U/pfx-$base" "$compat/pfx"
  put_dlls "$wined3d" "$wow64cpu" "$compat/pfx"
  : >"$out/$run.phases"
  echo "label=$label run=$run wined3d=$wined3d wow64cpu=$wow64cpu base=$base WINEDEBUG=${WINEDEBUG:-} MACRON_OPENGL=${MACRON_OPENGL:-} WINE_D3D_CONFIG=${WINE_D3D_CONFIG:-}" >"$out/$run.config"
  mark launch
  (
    cd "$game" && SteamAppId=871720 STEAM_COMPAT_DATA_PATH="$compat" \
      "$HOME/work/bin/macron" waitforexitandrun "$game/Ultimate Custom Night.exe" 2>&1 |
      perl -MTime::HiRes=time -ne 'printf "%.3f %s", time, $_' >"$out/$run.log"
  ) &
  sub=$!
  state="" t=0
  while [ -z "$(game_window)" ]; do
    sleep 1; t=$((t + 1))
    pgrep -f 'macron waitforexitandrun' >/dev/null || { state=exit-before-window; break; }
    [ "$t" -ge 240 ] && { state=no-window; break; }
  done
  if [ -z "$state" ]; then
    mark window
    game_window >"$out/$run.window"
    measure title
    # Keys reach the game only once Wine is the active app (a click alone, with
    # the Finder in front, is not enough), and now and then one is lost
    # anyway, so each step is checked by reading the screen (drive's OCR) and
    # tried again.
    if advance "click 512 300" "key return" -- gone "PRESS ENTER"; then
      mark enter
      measure menu
      # GO! on the menu opens the controls screen, which has its own GO!.
      if advance "click 940 690" -- shown "Power Generator"; then
        mark go
        "$drive" shot wine "Ultimate Custom Night" "$out/$run-controls.png" >/dev/null 2>&1
        if advance "click 928 650" -- gone "Power Generator"; then
          mark go2
          measure night "${UCN_NIGHT_WARM:-25}"
        else state=stuck-on-controls; fi
      else state=stuck-on-menu; fi
    else state=stuck-on-title; fi
    pgrep -f 'Custom Night.exe' >/dev/null || state=died
  fi
  mark stop
  kill_wine
  for i in $(seq 1 20); do kill -0 "$sub" 2>/dev/null || break; sleep 1; done
  kill -9 "$sub" 2>/dev/null
  rm -rf "$compat"
  python3 -I "$(dirname "$0")/ucn-summary.py" result "$out" "$run" "${state:-ok}" >>"$out/results.jsonl"
  tail -1 "$out/results.jsonl"
done
echo done >"$out/done"
