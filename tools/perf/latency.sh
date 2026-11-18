#!/bin/sh
# How far a Direct3D 11 program runs ahead of the GPU under Macron's DXMT,
# with the settings that change it, on one Mac.
#
#   tools/perf/latency.sh <gpuload.exe> <out-dir> [seconds]
#
# Runs tools/perf/gpuload.c through `macron waitforexitandrun` for each load
# and each setting below, in alternating order, twice, and prints one line a
# run: the program's own frame rate, the frames queued between its input and
# the screen (gpuload's RESULT queue), and DXMT's own reading of the same
# (dxmt.logLatency, the median of its lines). Everything is kept in <out-dir>.
#
# Loads: `gpu` is limited by the GPU (full-screen passes, little CPU work),
# `cpu` by the program itself (many small draws), `sync` is `gpu` presented
# with vsync. Settings: `off` (no sleep, the frame latency limit as the
# program left it; DXMT 0.80 as released differs from it only with vsync,
# where it does not count a frame until it is on screen), `sleep` (Macron's
# default: a frame waits for the encoder, never for the GPU), `limit1` (no
# sleep, dxgi.maxFrameLatency=1). SETTINGS names others, as "off sleep
# finish": `limit2`, `nosync` (the sleep, but not with vsync), `finish` and
# `start` (the sleep, also for the GPU, with dxmt.latencyGpuModel set to
# that). LOADS names the loads to run. FRAMES=1 adds DXMT's line a frame
# (dxmt.logLatencyFrames) to each run's launch log.
#
# A Mac without a fan slows down within a minute of such a load, and a run
# made while it is slow says nothing about the setting. So each run first
# waits for macOS's thermal pressure level to be 0 (up to COOL seconds,
# default 180) and its line ends with the level before and after it; compare
# only runs that began and ended at 0.
#
# Needs macron on PATH with its runtime installed, and a logged-in desktop.
# PASSES and DRAWS change the two loads (a slower GPU needs fewer passes).
set -eu

[ $# -ge 2 ] || { echo "usage: latency.sh <gpuload.exe> <out-dir> [seconds]" >&2; exit 2; }
exe=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
mkdir -p "$2"
out=$(cd "$2" && pwd)
seconds=${3:-15}
passes=${PASSES:-12}
draws=${DRAWS:-6000}
app_id=990004
frames=False
[ "${FRAMES:-0}" = 1 ] && frames=True
logs="${MACRON_HOME:-$HOME/Library/Application Support/Macron}/logs/steam-$app_id"

load_args() {
	case $1 in
	gpu) echo "--mode fill --passes $passes --sync 0 --work 2" ;;
	cpu) echo "--mode draws --draws $draws --sync 0" ;;
	sync) echo "--mode fill --passes $passes --sync 1 --work 2" ;;
	esac
}

setting_config() {
	case $1 in
	off) echo "dxmt.latencySleep=False" ;;
	sleep) echo "dxmt.latencySleep=True" ;;
	nosync) echo "dxmt.latencySleep=True;dxmt.latencySleepWithVsync=False" ;;
	finish) echo "dxmt.latencySleep=True;dxmt.latencyGpuModel=finish" ;;
	start) echo "dxmt.latencySleep=True;dxmt.latencyGpuModel=start" ;;
	limit1) echo "dxmt.latencySleep=False;dxgi.maxFrameLatency=1" ;;
	limit2) echo "dxmt.latencySleep=False;dxgi.maxFrameLatency=2" ;;
	esac
}

thermal() {
	notifyutil -g com.apple.system.thermalpressurelevel 2>/dev/null | awk '{print $2}'
}

wait_cool() {
	waited=0
	while [ "$(thermal)" != 0 ] && [ -n "$(thermal)" ] && [ $waited -lt "${COOL:-180}" ]; do
		sleep 5
		waited=$((waited + 5))
	done
}

run() { # <load> <setting> <round>
	tag=$1-$2-$3
	rm -f "$out/$tag.txt"
	wait_cool
	before=$(thermal)
	# shellcheck disable=SC2046
	env DXMT_CONFIG="dxmt.logLatency=True;dxmt.logLatencyFrames=$frames;$(setting_config "$2")" SteamAppId=$app_id STEAM_COMPAT_DATA_PATH="$out/prefix" \
		macron waitforexitandrun "$exe" $(load_args "$1") --size 1280x720 --seconds "$seconds" --out "$out/$tag.txt" \
		>"$out/$tag.macron.log" 2>&1 || true
	newest=$(ls -t "$logs"/*.log 2>/dev/null | head -1)
	[ -n "$newest" ] && cp "$newest" "$out/$tag.launch.log"
	fps=$(awk '/^RESULT fps/ {print $3}' "$out/$tag.txt" 2>/dev/null)
	queue=$(awk '/^RESULT queue/ {print $5 " frames, start to GPU done p50 " $10 " ms, p95 " $12}' "$out/$tag.txt" 2>/dev/null)
	dxmt=$(grep -o 'start to GPU done [0-9.]* ms' "$out/$tag.launch.log" 2>/dev/null | awk '{print $5}' | sort -n | awk '{v[NR] = $1} END {if (NR) print v[int((NR + 1) / 2)] " ms over " NR " lines"}')
	printf '%-5s %-7s %s  fps %-8s queue %s; DXMT says %s; thermal %s to %s\n' "$1" "$2" "$3" "${fps:-none}" "${queue:-none}" "${dxmt:-nothing}" "${before:--}" "$(thermal)"
}

for round in 1 2; do
	for load in ${LOADS:-gpu cpu sync}; do
		order=${SETTINGS:-off sleep limit1}
		[ $round = 1 ] || order=$(echo "$order" | awk '{for (i = NF; i > 0; i--) printf "%s ", $i}')
		for setting in $order; do
			run $load $setting $round
		done
	done
done | tee "$out/summary.txt"
