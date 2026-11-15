#!/bin/bash
# ucn-play.sh <label>: Ultimate Custom Night started with Steam's Play button
# (ci/steam-play/steam.sh), with macron's frame counter on (the show-fps file),
# measured at the title screen, the character-select menu and a night the way
# ucn-run.sh does (settle, then UCN_MEASURE seconds), stopped with Steam's STOP.
# The frame rate is wined3d's own counter in macron's launch log
# (trace:fps ... @ approx N fps); each place's lines are those written while it
# was measured. Needs DRIVE, EVIDENCE and STEAM_TEST_ACCOUNT as steam.sh does,
# Steam signed in with Steam Play on, and the game installed.
# Results: $EVIDENCE/<label>/{phases,fps-<place>.txt,<place>.png,summary.txt}
set -u
label=$1
here=$(cd "$(dirname "$0")" && pwd)
steam=$here/../../ci/steam-play/steam.sh
out=$EVIDENCE/$label
mkdir -p "$out"
M="$HOME/Library/Application Support/Macron"
meas=${UCN_MEASURE:-20}
touch "$M/show-fps"
game_title="Ultimate Custom Night"

EVIDENCE=$out "$steam" play 871720 wine "$game_title" 300 || exit 1
log=$(ls -t "$M"/logs/steam-871720/*.log | head -1)
echo "log $log" >"$out/phases"
mark() { echo "$1 $(grep -c . "$log")" >>"$out/phases"; }

advance() { # <drive command>... -- shown|gone <text>
  local cmds=() want text
  while [ "$1" != -- ]; do cmds+=("$1"); shift; done
  want=$2 text=$3
  for _ in 1 2 3 4 5; do
    "$DRIVE" activate wine >/dev/null 2>&1; sleep 1
    for c in "${cmds[@]}"; do "$DRIVE" $c >/dev/null 2>&1; sleep 1; done
    for _ in 1 2 3 4 5 6; do
      sleep 2
      if "$DRIVE" find wine "$game_title" "$text" >/dev/null 2>&1; then [ "$want" = shown ] && return 0
      else [ "$want" = gone ] && return 0; fi
    done
  done
  return 1
}

measure() { # <place> <settle>
  # steam.sh leaves Steam in front after Play, and macOS throttles a
  # background app: the game must be the front app while it is measured.
  "$DRIVE" activate wine >/dev/null 2>&1
  sleep "$2"
  "$DRIVE" activate wine >/dev/null 2>&1
  mark "$1-start"; sleep "$meas"; mark "$1-end"
  "$DRIVE" shot wine "$game_title" "$out/$1.png" >/dev/null 2>&1
}

state=ok
measure title 15
if advance "click 512 300" "key return" -- gone "PRESS ENTER"; then
  measure menu 15
  if advance "click 940 690" -- shown "Power Generator" && advance "click 928 650" -- gone "Power Generator"; then
    measure night 25
  else state=stuck-before-night; fi
else state=stuck-on-title; fi
EVIDENCE=$out "$steam" stop 871720 wine "$game_title" || state="$state, stop failed"
rm -f "$M/show-fps"

python3 -I - "$out/phases" "$state" >"$out/summary.txt" <<'PY'
import re, statistics, sys
lines = open(sys.argv[1]).read().split("\n")
log = lines[0][4:]
marks = dict((l.split()[0], int(l.split()[1])) for l in lines[1:] if l.strip())
text = open(log, errors="replace").read().split("\n")
print("state", sys.argv[2])
for place in ("title", "menu", "night"):
    if place + "-start" not in marks or place + "-end" not in marks:
        print(place, "not measured"); continue
    seg = text[marks[place + "-start"]:marks[place + "-end"]]
    fps = [float(m.group(1)) for l in seg for m in [re.search(r"@ approx ([0-9.]+)fps", l)] if m]
    if fps:
        print(f"{place}: median {statistics.median(fps):.2f} fps, min {min(fps):.2f}, max {max(fps):.2f}, {len(fps)} samples")
    else:
        print(f"{place}: no frame counter lines (wined3d did not draw this place)")
PY
cat "$out/summary.txt"
