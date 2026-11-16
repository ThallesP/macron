#!/bin/sh
# Counts the Wine server requests a running game makes, from a macron launch
# log written with WINEDEBUG=+server (wineserver started with -d then writes
# "<thread>: <request>( <args> )" for every request and
# "<thread>: <request>() = <status> ..." for every reply to its stderr, which
# is macron's log).
#
#   tools/perf/server-count.sh <log> <seconds> <out-dir>
#
# Cuts the next <seconds> of the log as it grows, and writes into <out-dir>:
# requests.txt (count by request type), threads.txt (count by thread),
# thread-requests.txt (count by thread and type), selects.txt (what the
# select requests wait for: WAIT, WAIT_ALL, SIGNAL_AND_WAIT, ... and whether
# they have a timeout), and window.txt (bytes, lines, requests, frames by the
# Metal HUD in the slice if it was on). Request names and counts only: no
# arguments, which can hold paths or names, leave the Mac.
set -eu
[ $# -eq 3 ] || {
	echo "usage: server-count.sh <log> <seconds> <out-dir>" >&2
	exit 2
}
log=$1 seconds=$2 out=$3
mkdir -p "$out"
start=$(stat -f %z "$log")
sleep "$seconds"
end=$(stat -f %z "$log")
slice="$out/slice.tmp"
tail -c +"$((start + 1))" "$log" | head -c "$((end - start))" >"$slice"

requests() { grep -E '^[0-9a-f]+: [a-z_0-9]+\( ' "$slice" | grep -v '() = ' || true; }
requests | sed -E 's/^([0-9a-f]+): ([a-z_0-9]+)\(.*/\2/' | sort | uniq -c | sort -rn >"$out/requests.txt"
requests | sed -E 's/^([0-9a-f]+): .*/\1/' | sort | uniq -c | sort -rn | head -20 >"$out/threads.txt"
requests | sed -E 's/^([0-9a-f]+): ([a-z_0-9]+)\(.*/\1 \2/' | sort | uniq -c | sort -rn | head -40 >"$out/thread-requests.txt"
grep -E '^[0-9a-f]+: select\( ' "$slice" | sed -E -n 's/.*timeout=([^,]*),.*data=\{([A-Z_]+).*/\2 timeout=\1/p' |
	sed -E 's/timeout=(infinite|0)$/timeout=\1/; s/timeout=[^i0].*/timeout=other/' | sort | uniq -c | sort -rn >"$out/selects.txt"
{
	echo "seconds $seconds"
	echo "bytes $((end - start))"
	echo "lines $(wc -l <"$slice" | tr -d ' ')"
	echo "requests $(requests | wc -l | tr -d ' ')"
	echo "metal-HUD lines $(grep -c 'metal-HUD' "$slice" || true)"
} >"$out/window.txt"
rm -f "$slice"
cat "$out/window.txt"
head -15 "$out/requests.txt"
