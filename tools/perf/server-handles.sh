#!/bin/sh
# Which objects a polling thread polls: from a macron launch log written
# with WINEDEBUG=+server, takes the next <seconds> of the log, finds the
# thread that made the most zero-timeout waits, counts the handles it waited
# on, and for the most polled ones finds, earlier in the log, the request
# whose reply returned that handle (create_event, create_semaphore,
# open_mutex, dup_handle, new_thread, ...), which says what kind of object
# it is.
#
#   tools/perf/server-handles.sh <log> <seconds> <out-dir>
#
# Writes <out-dir>/handles.txt: per handle, how often it was polled and the
# request that made it. Request names, handle numbers and counts only.
set -eu
[ $# -eq 3 ] || {
	echo "usage: server-handles.sh <log> <seconds> <out-dir>" >&2
	exit 2
}
log=$1 seconds=$2 out=$3
mkdir -p "$out"
start=$(stat -f %z "$log")
sleep "$seconds"
end=$(stat -f %z "$log")
slice="$out/slice.tmp"
tail -c +"$((start + 1))" "$log" | head -c "$((end - start))" >"$slice"

polls() { grep -E '^[0-9a-f]+: select\( .*timeout=0,' "$slice" || true; }
thread=$(polls | sed -E 's/^([0-9a-f]+):.*/\1/' | sort | uniq -c | sort -rn | head -1 | awk '{print $2}')
[ -n "$thread" ] || {
	echo "no zero-timeout waits in the slice" | tee "$out/handles.txt"
	rm -f "$slice"
	exit 0
}
polls | grep "^$thread: " | sed -E -n 's/.*data=\{[A-Z_]+,handles=\{([0-9a-f,]+)\}.*/\1/p' |
	tr ',' '\n' | sort | uniq -c | sort -rn | head -8 >"$out/polled.tmp"
{
	echo "thread $thread, zero-timeout waits in $seconds s: $(polls | grep -c "^$thread: ")"
	echo "polls handle  made by (the last reply returning it before the slice)"
	while read -r count handle; do
		maker=$(head -c "$start" "$log" | grep -E "^[0-9a-f]+: [a-z_0-9]+\(\) = [A-Z_0-9]+ \{.*handle=$handle[,} ]" | tail -1 |
			sed -E 's/^[0-9a-f]+: ([a-z_0-9]+)\(\).*/\1/')
		echo "$count $handle ${maker:-not found}"
	done <"$out/polled.tmp"
} >"$out/handles.txt"
rm -f "$slice" "$out/polled.tmp"
cat "$out/handles.txt"
