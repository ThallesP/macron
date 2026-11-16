#!/bin/bash
# ow-run.sh <label> <play|shell> <seconds> [NAME=value ...]
# Starts Overwatch (Steam app 2357570) on a sandbox signed in to the test
# account, follows its memory with ow-mon.sh for <seconds>, takes a picture of
# the screen, and stops it.
#   play:  Steam's Play button (ci/steam-play/steam.sh play), stopped with STOP.
#   shell: macron waitforexitandrun from a shell, the way Steam starts it, with
#          the NAME=value pairs added to its environment; stopped with
#          wineserver -k.
# The game is kept the front application: macOS throttles a background app.
# Needs DRIVE and EVIDENCE as steam.sh does. Results: $EVIDENCE/<label>/
set -u
label=$1 mode=$2 seconds=$3
shift 3
here=$(cd "$(dirname "$0")" && pwd)
steam=$here/../../ci/steam-play/steam.sh
out=$EVIDENCE/$label
mkdir -p "$out"
steam_dir="$HOME/Library/Application Support/Steam"
game_dir="$steam_dir/steamapps/common/Overwatch"
compat="$steam_dir/steamapps/compatdata/2357570"
logs="$HOME/Library/Application Support/Macron/logs/steam-2357570"

"$here/ow-mon.sh" "$out" "$seconds" &
mon=$!
date -u +%FT%TZ > "$out/started"
case "$mode" in
  play)
    # The game's window opens in its own full-screen Space and is not listed
    # under that title, so steam.sh's wait for it runs out: Play is pressed
    # in the background and the game is followed by its process instead.
    (EVIDENCE=$out "$steam" play 2357570 wine Overwatch 300 > "$out/play.txt" 2>&1 &)
    ;;
  shell)
    printf '%s\n' "$@" > "$out/env"
    (cd "$game_dir" && env "$@" SteamAppId=2357570 SteamGameId=2357570 STEAM_COMPAT_DATA_PATH="$compat" \
      STEAM_COMPAT_CLIENT_INSTALL_PATH="$steam_dir" \
      nohup /usr/local/bin/macron waitforexitandrun "$game_dir/Overwatch.exe" > "$out/macron-stdout.txt" 2>&1 &)
    ;;
  *) echo "mode is play or shell" >&2; exit 2 ;;
esac

# ow-mon.sh counts <seconds> from when the game's process appears and then
# exits; until then the game is kept in front. The picture is taken just
# before the end, and with OW_SHOTS=1 also every 30 s (screen-<t>.png). The
# pictures can show the Steam profile name and a Battle.net link code: they
# are for looking at on the sandbox, never for the repository.
# A background process cannot activate another app on macOS, so `drive
# activate` does not bring the game forward once Steam is in front; a click on
# the game's Dock icon does. OW_DOCK=x,y is where that icon is (on a fresh
# sandbox's 1024x768 screen, right of Steam's: 835,738); check it on the
# pictures. It is clicked only while Steam's window is on screen: the game
# runs in its own full-screen Space, where that point is inside the game.
front() {
  "$DRIVE" activate wine > /dev/null 2>&1
  [ -n "${OW_DOCK:-}" ] || return 0
  "$DRIVE" windows | awk -F'\t' '$4 == "Steam Helper" && $2 == 0 { split($3, p, " "); split(p[2], wh, "x"); if (wh[1] >= 600) found = 1 } END { exit !found }' || return 0
  "$DRIVE" click "${OW_DOCK%,*}" "${OW_DOCK#*,}" > /dev/null 2>&1
  echo "$(date -u +%T) clicked the game's Dock icon" >> "$out/front.log"
}
# OW_SAMPLE="150 280" takes a 10 s `sample` of the game's process at those
# seconds (sample-<t>.txt), to see where its threads are.
last_shot=-30
samples=" ${OW_SAMPLE:-} "
while kill -0 "$mon" 2> /dev/null; do
  front
  t=$(tail -1 "$out/mon.tsv" 2> /dev/null | cut -f1)
  if [ -n "$t" ] && [ "$t" != t ]; then
    for at in $samples; do
      if [ "$t" -ge "$at" ]; then
        samples=${samples/ $at / }
        pid=$(awk 'NR == 1 { print $2 }' "$out/pid")
        (sample "$pid" 10 -file "$out/sample-$at.txt" > /dev/null 2>&1 &)
      fi
    done
    if [ "${OW_SHOTS:-0}" = 1 ] && [ $((t - last_shot)) -ge 30 ]; then
      last_shot=$t
      screencapture -x "$out/screen-$t.png"
    fi
    if [ "$t" -ge $((seconds - 30)) ] && [ ! -f "$out/screen.png" ]; then
      sleep 2
      screencapture -x "$out/screen.png"
    fi
  fi
  sleep 5
done
wait "$mon"

# The newest launch log is this run's.
log=$(ls -t "$logs"/*.log 2> /dev/null | head -1)
[ -n "$log" ] && grep -v '^\s*VK_\|^\s*$' "$log" | head -400 > "$out/macron.log"

case "$mode" in
  play) EVIDENCE=$out "$steam" stop 2357570 wine Overwatch || echo "steam.sh stop failed" >> "$out/started" ;;
  *)
    wineserver=$(ls /Users/runner/Library/Application\ Support/Macron/runtimes/*/wine-dxmt/bin/wineserver 2> /dev/null | head -1)
    WINEPREFIX="$compat/pfx" "${wineserver:-wineserver}" -k
    ;;
esac
for _ in $(seq 1 30); do pgrep -x wine > /dev/null || break; sleep 2; done
pgrep -lx wine && echo "wine still running after the stop" >> "$out/started"
echo "stopped $(date -u +%FT%TZ)" >> "$out/started"
