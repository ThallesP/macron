#!/bin/bash
# Starts a program through `macron launch` again and again on a Mac and counts
# how each start ended: ok (exit 0), crash (Wine reported an unhandled
# exception), exit N, or stall (killed after the timeout). Every Wine process
# is killed between starts. Run it detached: the sandbox tunnel stalls.
# Usage: loop.sh <label> <starts> <timeout-s> <game> <exe> [args...]
# Environment: MACRON (the macron binary), OUT (results folder), plus anything
# macron or Wine reads (MACRON_OPENGL, WINEDEBUG, WINE_D3D_CONFIG).
set -u
label=$1 starts=$2 limit=$3 game=$4 exe=$5
shift 5
out=${OUT:-/tmp/loop}
mkdir -p "$out/$label"
summary="$out/$label.txt"
: >"$summary"

kill_wine() {
  "$MACRON" kill --game "$game" >/dev/null 2>&1
  # Wine's processes run from the runtime or show a Windows path; a bare
  # '\.exe' would also kill a compiler writing conftest.exe.
  pkill -9 -f 'Macron/runtimes/' 2>/dev/null
  pkill -9 -f '^[A-Za-z]:\\' 2>/dev/null
  sleep 1
}

for i in $(seq 1 "$starts"); do
  kill_wine
  log="$out/$label/$i.log"
  start=$(date +%s)
  "$MACRON" launch --game "$game" -- "$exe" "$@" >"$log" 2>&1 &
  pid=$!
  state=""
  while kill -0 "$pid" 2>/dev/null; do
    if [ $(($(date +%s) - start)) -ge "$limit" ]; then
      state=stall
      kill_wine
      kill -9 "$pid" 2>/dev/null
      break
    fi
    sleep 1
  done
  wait "$pid"
  code=$?
  seconds=$(($(date +%s) - start))
  if [ -z "$state" ]; then
    if grep -q "Unhandled\|unhandled exception" "$log"; then
      state=crash
    elif [ "$code" = 0 ]; then
      state=ok
    else
      state="exit$code"
    fi
  fi
  fault=$(grep -m1 -o "Unhandled [^\r]*" "$log" | head -c 160)
  echo "$label $i $state $code ${seconds}s $fault" | tee -a "$summary"
done
kill_wine
printf '%s: ' "$label" >>"$out/totals.txt"
awk '{print $3}' "$summary" | sort | uniq -c | tr '\n' ' ' >>"$out/totals.txt"
echo >>"$out/totals.txt"
