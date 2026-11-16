#!/bin/bash
# GOAL.md gate 2, end to end, on a Mac where nobody works: Steam from Valve's
# DMG in /Applications (already started once), signed in to the test account,
# then Macron. Every step goes through Steam's own window (steam.sh) and keeps
# a masked screenshot in EVIDENCE.
#
#   DRIVE=<drive> EVIDENCE=<dir> STEAM_TEST_ACCOUNT=<file> MACRON=<macron> gate2.sh
#
# GATE2_PHASE splits the run for gate3.sh, where Macron's installer does the
# enabling: "before" stops after the checks on a stock Steam, "after" starts
# at "Steam Play is on" and stops before the unknown-build check and the
# disable. STEAMPLAY_SOURCE is the steamplay/ folder, needed for the
# unknown-build check; without it that check is skipped and says so.
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
drive=${DRIVE:?}
evidence=${EVIDENCE:?}
phase=${GATE2_PHASE:-all}
macron=${MACRON:-}
[ "$phase" = before ] || [ -n "$macron" ] || { echo "MACRON must point at the macron program" >&2; exit 2; }
steam_app=${STEAM_APP:-/Applications/Steam.app}
steam_dir="$HOME/Library/Application Support/Steam"
macron_home="$HOME/Library/Application Support/Macron"
windows_game=871720
windows_game_window=("wine" "Ultimate Custom Night")
native_game=404410
native_game_window=("Endless Sky" "Endless Sky")
helper="Steam Helper"
mkdir -p "$evidence"

steam() {
  "$here/steam.sh" "$@" || exit 1
}

say() {
  echo
  echo "== $*"
}

fail() {
  echo "::error::gate2: $*"
  exit 1
}

sees() {
  "$drive" find "$helper" Steam "$1" > /dev/null 2>&1
}

app_listing() {
  (cd "$steam_app" && find . -type f -print0 | xargs -0 shasum -a 256 | sort -k 2)
}

lines_of() {
  wc -l < "$1" 2> /dev/null || echo 0
}

since() {
  tail -n +"$(($2 + 1))" "$1" 2> /dev/null
}

if [ "$phase" != after ]; then
  say "Steam, signed in to the test account"
  pgrep -x steam_osx > /dev/null || steam start
  steam login
  steam add "$windows_game"
  steam add "$native_game"

  say "Before Macron: the Windows-only game cannot be installed, the Mac game installs and runs"
  if [ -n "$macron" ]; then
    "$macron" steam status | grep -q '^steam play:      off$' || fail "Steam Play is already on; this run starts from a stock Steam"
  fi
  steam stock "$windows_game" "1-before-macron-windows-game"
  [ -f "$steam_dir/steamapps/appmanifest_$native_game.acf" ] || EVIDENCE="$evidence/before-macron" steam install "$native_game" 1200
  EVIDENCE="$evidence/before-macron" steam play "$native_game" "${native_game_window[@]}" 180
  EVIDENCE="$evidence/before-macron" steam stop "$native_game" "${native_game_window[@]}"
  app_listing > "$evidence/steam-app-before.sha256"
  codesign --verify --strict --deep "$steam_app" || fail "Steam.app's signature is not valid before Macron touched it"
fi
if [ "$phase" = before ]; then
  say "Done with the stock Steam"
  exit 0
fi

if [ "$phase" = all ]; then
  say "macron steam enable"
  steam quit
  "$macron" steam enable | tee "$evidence/enable.txt"
  grep -q '^steam play:      on$' "$evidence/enable.txt" || fail "macron steam enable did not switch Steam Play on"
  steam start
fi

say "Steam Play is on in the running client"
switch_log="$macron_home/logs/steamplay.log"
tail -1 "$switch_log"
tail -1 "$switch_log" | grep -q 'Steam Play is on' || fail "the switch library did not report Steam Play on"
grep -q 'Registering tool macron' "$steam_dir/logs/compat_log.txt" || fail "Steam did not register the macron tool"

say "Install the Windows-only game with Steam's Install button"
steam install "$windows_game" 1800
ls "$steam_dir/steamapps/common/Ultimate Custom Night/"

say "Play starts it through macron"
compat_lines=$(lines_of "$steam_dir/logs/compat_log.txt")
steam play "$windows_game" "${windows_game_window[@]}" 420
since "$steam_dir/logs/compat_log.txt" "$compat_lines" | tee "$evidence/compat-log-play.txt"
grep -q "StartSession: appID $windows_game" "$evidence/compat-log-play.txt" || fail "Steam's compat log has no session for the game"
grep -q 'macron waitforexitandrun' "$evidence/compat-log-play.txt" || fail "Steam's compat log does not show macron waitforexitandrun"
game_log=$(ls -t "$macron_home/logs/steam-$windows_game/"*.log | head -1)
grep 'macron:' "$game_log" | tee "$evidence/macron-log-play.txt"
grep -Eq 'macron: .*starting Ultimate Custom Night.exe' "$evidence/macron-log-play.txt" || fail "macron's log does not show it starting the game"

say "Quitting the game returns Steam to Play"
steam quit_game "$windows_game" "${windows_game_window[@]}"

say "Steam's Stop button stops it too"
steam play "$windows_game" "${windows_game_window[@]}" 420
steam stop "$windows_game" "${windows_game_window[@]}"
sleep 5
pgrep -f 'Ultimate Custom Night.exe' > /dev/null && fail "the game's process is still alive after Stop"

say "The Mac-native game still runs as before, not through macron"
[ -f "$steam_dir/steamapps/appmanifest_$native_game.acf" ] || steam install "$native_game" 1200
process_lines=$(lines_of "$steam_dir/logs/gameprocess_log.txt")
steam play "$native_game" "${native_game_window[@]}" 180
since "$steam_dir/logs/gameprocess_log.txt" "$process_lines" | grep "AppID $native_game" | tee "$evidence/native-game-processes.txt"
grep -q 'Endless Sky.app/Contents/MacOS/Endless Sky' "$evidence/native-game-processes.txt" || fail "Steam did not start the Mac game's own program"
pgrep -f 'macron (waitforexitandrun|run)' > /dev/null && fail "macron is running for a Mac-native game"
steam stop "$native_game" "${native_game_window[@]}"

if [ "$phase" = after ]; then
  say "Done with Steam's buttons"
  exit 0
fi

say "A Steam build the switch does not know: it refuses and Steam stays stock"
steam quit
if [ -n "${STEAMPLAY_SOURCE:-}" ]; then
  unknown=$(mktemp -d)
  cp "$STEAMPLAY_SOURCE"/steamplay.c "$STEAMPLAY_SOURCE"/build.sh "$unknown/"
  sed -E 's/"[0-9a-f]{32}"/"00000000000000000000000000000000"/' "$STEAMPLAY_SOURCE/builds.h" > "$unknown/builds.h"
  "$unknown/build.sh" "$unknown/macron-steamplay.dylib"
  MACRON_STEAMPLAY_LIBRARY="$unknown/macron-steamplay.dylib" "$macron" steam enable > /dev/null || fail "could not install the switch library built for the check"
  switch_lines=$(lines_of "$switch_log")
  steam start
  since "$switch_log" "$switch_lines" | tee "$evidence/unknown-build.txt"
  grep -q 'UNKNOWN_STEAM_BUILD' "$evidence/unknown-build.txt" || fail "the switch did not refuse a build it does not know"
  steam stock "$windows_game" "unknown-build-windows-game"
  steam quit
else
  echo "skipped: STEAMPLAY_SOURCE is not set"
fi

say "macron steam disable puts Steam back"
"$macron" steam disable | tee "$evidence/disable.txt"
grep -q "Valve's files are back, byte for byte" "$evidence/disable.txt" || fail "macron steam disable did not restore Valve's files"
app_listing > "$evidence/steam-app-after.sha256"
diff "$evidence/steam-app-before.sha256" "$evidence/steam-app-after.sha256" || fail "Steam.app differs from before Macron"
codesign --verify --strict --deep "$steam_app" || fail "Steam.app's signature is not valid after the disable"
spctl -a -vv "$steam_app" 2>&1 | tee "$evidence/gatekeeper-after.txt"
[ -e "$steam_dir/compatibilitytools.d/macron" ] && fail "macron's tool folder is still in Steam"
steam start
steam stock "$windows_game" "9-after-disable-windows-game"
say "Gate 2 flow passed"
