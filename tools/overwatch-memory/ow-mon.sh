#!/bin/bash
# ow-mon.sh <out> <seconds>: follows Overwatch's process on a Mac while it runs.
# Every 3 s one line in <out>/mon.tsv: seconds since the start, the process's
# phys_footprint (MB, from footprint), swap used (MB), the memory pressure
# level, and the size of DXMT's shader cache for Overwatch (MB). Every 30 s a
# full `footprint` and `vmmap -summary` into <out>/fp-<t>.txt and vm-<t>.txt.
# Wine shows a process's Windows command line in ps: the game's process is the
# one whose command line starts with a drive letter and Overwatch.exe, not
# steam.exe (whose command line also names Overwatch.exe) or macron.
set -u
out=$1 seconds=$2
mkdir -p "$out"
cache_dir="$(getconf DARWIN_USER_CACHE_DIR)dxmt/Overwatch.exe"

game_pid() {
  ps -axo pid=,command= | awk '{ pid = $1; $1 = ""; sub(/^ /, "") } /^"?[A-Za-z]:\\.*Overwatch\.exe/ && !/steam\.exe/ { print pid; found = 1; exit } END { exit !found }'
}

start=$(date +%s)
pid=
until pid=$(game_pid); do
  [ $(($(date +%s) - start)) -gt 600 ] && { echo "no Overwatch process within 600 s" >&2; exit 1; }
  sleep 1
done
echo "pid $pid found $(($(date +%s) - start)) s after the start" > "$out/pid"
ps -o pid,ppid,command -p "$pid" >> "$out/pid"
printf 't\tfootprint_mb\tswap_mb\tpressure\tdxmt_cache_mb\n' > "$out/mon.tsv"
t0=$(date +%s)
last_full=-30
while kill -0 "$pid" 2> /dev/null; do
  t=$(($(date +%s) - t0))
  [ "$t" -gt "$seconds" ] && break
  fp=$(footprint --noCategories -f bytes "$pid" 2> /dev/null | awk '/phys_footprint:/ { printf "%d", $2 / 1048576; exit }')
  swap=$(sysctl -n vm.swapusage | awk '{ for (i = 1; i <= NF; i++) if ($i == "used") { v = $(i + 2); sub(/M/, "", v); printf "%d", v } }')
  pressure=$(sysctl -n kern.memorystatus_vm_pressure_level)
  cache=$(du -sm "$cache_dir" 2> /dev/null | cut -f1)
  printf '%s\t%s\t%s\t%s\t%s\n' "$t" "${fp:-}" "${swap:-}" "$pressure" "${cache:-0}" >> "$out/mon.tsv"
  if [ $((t - last_full)) -ge 30 ]; then
    last_full=$t
    footprint "$pid" > "$out/fp-$t.txt" 2>&1
    vmmap -summary "$pid" > "$out/vm-$t.txt" 2>&1
  fi
  sleep 3
done
echo "done at $(($(date +%s) - t0)) s" >> "$out/pid"
