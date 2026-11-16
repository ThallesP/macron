#!/bin/sh
# A Direct3D 11 benchmark through Macron's DXMT, with no account, no input
# and a frame-time log. For real Macs: a sandbox's virtual GPU cannot run
# these benchmarks (docs/perf/README.md).
#
#   tools/perf/bench.sh setup <bench>
#   tools/perf/bench.sh run <bench> <tag> [WIDTHxHEIGHT] [windowed|fullscreen] [NAME=VALUE ...]
#   tools/perf/bench.sh summary <tag>
#   tools/perf/bench.sh pair <tag> [WIDTHxHEIGHT]
#
# <bench> is superposition (Unigine Superposition 1.1, 64-bit, 2017, the
# closer load to Overwatch) or heaven (Unigine Heaven 4.0, 32-bit, 2013).
# Both are free downloads from Unigine; nothing of them is redistributed.
#
# `run` starts the benchmark's own automated run, which plays a fixed camera
# path and quits by itself, through `macron waitforexitandrun` (the default
# DXMT profile, as an unknown Steam app gets), with Apple's Metal HUD logging
# every frame's interval and GPU time (MTL_HUD_ENABLED=1,
# MTL_HUD_LOG_ENABLED=1). NAME=VALUE pairs are added to the environment, so
# experiments are switches like DXMT_METALFX_SPATIAL_SWAPCHAIN=1.
#
# Everything goes to $MACRON_PERF_HOME (default ~/macron-perf): the
# installers, one Windows environment per benchmark, and results/<tag>/ with
# macron.log (macron's output, with the HUD lines), hud.csv (frame, interval
# ms, GPU ms), thermal.txt (the thermal pressure level each second),
# bench.txt (the benchmark's own result), engine-log.txt (the engine's log)
# and summary.txt. Each run first waits for thermal pressure level 0.
#
# `pair` runs tools/perf/gpuload.c through DXMT and its native Metal twin
# tools/perf/gpuload.m on the same Mac, for three loads (full-screen shader
# passes, many small draws, both), and prints a table: the GPU time a frame
# DXMT adds to the same work.
#
# Needs macron (Macron 0.4.0 or later) on PATH, curl, shasum, awk.
set -eu

home=${MACRON_PERF_HOME:-$HOME/macron-perf}

die() {
	echo "bench: $*" >&2
	exit 1
}

bench_vars() {
	case $1 in
	superposition)
		url=https://assets.unigine.com/d/Unigine_Superposition-1.1.exe
		sha=de5dbf4f7ad284b709c5db318dbedc1b025a41e50326595a444ee9cbd4a17182
		app_id=990010
		dir=Superposition
		exe=superposition.exe
		overrides=d3dcompiler_47
		;;
	heaven)
		url=https://assets.unigine.com/d/Unigine_Heaven-4.0.exe
		sha=497865a09af205a7d2e8812bfc2dd54870216cc1e95541b63cef89a4433b0e5f
		app_id=990020
		dir=Heaven
		exe=Heaven.exe
		overrides="d3dcompiler_42 d3dx11_42 d3dx9_42"
		;;
	*) die "unknown benchmark '$1': superposition or heaven" ;;
	esac
	compat="$home/$1"
	drive_c="$compat/pfx/drive_c"
}

# Runs a Windows program in the benchmark's Windows environment the way
# Steam starts a game, so the installed macron needs nothing new.
in_prefix() {
	SteamAppId=$app_id STEAM_COMPAT_DATA_PATH=$compat macron waitforexitandrun "$@"
}

setup() {
	bench_vars "$1"
	mkdir -p "$home/downloads" "$compat"
	installer="$home/downloads/$(basename "$url")"
	if ! echo "$sha  $installer" | shasum -a 256 -c >/dev/null 2>&1; then
		echo "downloading $url"
		curl -fsSL --retry 3 -o "$installer.part" "$url"
		echo "$sha  $installer.part" | shasum -a 256 -c >/dev/null || die "$installer: SHA-256 differs from $sha"
		mv "$installer.part" "$installer"
	fi
	if [ ! -f "$drive_c/$dir/bin/$exe" ]; then
		echo "installing into $compat (a minute or two)"
		in_prefix "$installer" /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP- "/DIR=C:\\$dir" >"$compat/install.log" 2>&1 ||
			die "the installer failed; see $compat/install.log"
		[ -f "$drive_c/$dir/bin/$exe" ] || die "the installer left no $dir/bin/$exe; see $compat/install.log"
	fi
	# Wine's own HLSL compiler cannot compile these engines' shaders
	# (vkd3d-shader: "Double-to-integer conversion" not implemented), so
	# the benchmark's own Microsoft compiler DLLs are used.
	for dll in $overrides; do
		in_prefix "$drive_c/windows/system32/reg.exe" add 'HKCU\Software\Wine\DllOverrides' /v "$dll" /d native /f >/dev/null 2>&1 ||
			die "could not set $dll to native"
	done
	echo "ready: $1 in $compat"
}

# A MacBook Air has no fan: start each run from a cool machine, so runs
# compare. Waits up to 10 minutes for thermal pressure level 0 (nominal).
wait_cool() {
	waited=0
	while [ "$(notifyutil -g com.apple.system.thermalpressurelevel 2>/dev/null | awk '{print $2}')" != 0 ] && [ $waited -lt 600 ]; do
		[ $waited = 0 ] && echo "waiting for the Mac to cool down (thermal pressure above nominal)"
		sleep 10
		waited=$((waited + 10))
	done
	echo "$waited"
}

thermal_logger() {
	while :; do
		printf '%s %s\n' "$(date +%H:%M:%S)" "$(notifyutil -g com.apple.system.thermalpressurelevel 2>/dev/null | awk '{print $2}')"
		sleep 1
	done
}

run() {
	bench=$1 tag=$2
	shift 2
	size=1600x900
	mode=windowed
	case ${1:-} in *x*) size=$1 && shift ;; esac
	case ${1:-} in windowed | fullscreen) mode=$1 && shift ;; esac
	bench_vars "$bench"
	[ -f "$drive_c/$dir/bin/$exe" ] || die "run 'tools/perf/bench.sh setup $bench' first"
	width=${size%x*} height=${size#*x}
	fullscreen=0
	[ "$mode" = fullscreen ] && fullscreen=1
	out="$home/results/$tag"
	[ -e "$out" ] && die "$out exists; pick another tag"
	mkdir -p "$out"

	case $bench in
	superposition)
		rm -rf "$drive_c"/users/*/Superposition/automation "$drive_c"/users/*/Superposition/log.html
		set -- "$@" -- -sound_app openal -system_script superposition/system_script.cpp -data_path ../ \
			-engine_config ../data/superposition/unigine.cfg -video_mode -1 -project_name Superposition -video_resizable 1 \
			-console_command "config_readonly 1 && world_load superposition/superposition" \
			-mode 2 -preset 0 -video_app direct3d11 -video_fullscreen "$fullscreen" \
			-video_width "$width" -video_height "$height" -shaders_quality 1 -textures_quality 1
		;;
	heaven)
		rm -f "$drive_c/$dir/bin/log.html"
		set -- "$@" -- -data_path ../ -sound_app null -engine_config ../data/heaven_4.0.cfg -system_script heaven/unigine.cpp \
			-video_mode -1 -extern_define PHORONIX,RELEASE -video_app direct3d11 -video_fullscreen "$fullscreen" \
			-video_width "$width" -video_height "$height"
		;;
	esac

	{
		echo "bench $bench, $size $mode, $(date -u +%Y-%m-%dT%H:%M:%SZ)"
		echo "macOS $(sw_vers -productVersion), $(sysctl -n machdep.cpu.brand_string), $(macron --version 2>/dev/null || true)"
		pmset -g batt | sed -n 1p
	} >"$out/env.txt"
	extra=
	while [ "$1" != -- ]; do
		case $1 in *=*) ;; *) die "'$1' is not NAME=VALUE" ;; esac
		extra="$extra $1"
		echo "env $1" >>"$out/env.txt"
		shift
	done
	shift

	echo "waited $(wait_cool) s for thermal pressure 0 before starting" >>"$out/env.txt"
	thermal_logger >"$out/thermal.txt" &
	thermal=$!
	trap 'kill $thermal ${watchdog:-} 2>/dev/null || true' EXIT
	# A benchmark that hangs (a GPU error, a dialog) must not hold up the
	# rest of the night: end it after BENCH_TIMEOUT seconds.
	(
		sleep "${BENCH_TIMEOUT:-1200}"
		echo "timed out after ${BENCH_TIMEOUT:-1200} s: ended $exe" >>"$out/env.txt"
		pkill -f "$exe"
	) &
	watchdog=$!
	started=$(date +%s)
	cd "$drive_c/$dir/bin"
	# shellcheck disable=SC2086 # $extra is a list of NAME=VALUE words
	env MTL_HUD_ENABLED=${HUD:-1} MTL_HUD_LOG_ENABLED=${HUD:-1} $extra SteamAppId=$app_id STEAM_COMPAT_DATA_PATH="$compat" \
		macron waitforexitandrun "$PWD/$exe" "$@" >"$out/macron.log" 2>&1 || true
	echo "ran $(($(date +%s) - started)) s" >>"$out/env.txt"
	kill $thermal $watchdog 2>/dev/null || true

	case $bench in
	superposition)
		cat "$drive_c"/users/*/Superposition/automation/* >"$out/bench.txt" 2>/dev/null || true
		sed -e 's/<[^>]*>//g' "$drive_c"/users/*/Superposition/log.html >"$out/engine-log.txt" 2>/dev/null || true
		;;
	heaven)
		sed -e 's/<[^>]*>//g' "$drive_c/$dir/bin/log.html" >"$out/engine-log.txt" 2>/dev/null || true
		sed -e 's/<[^>]*>//g' "$drive_c/$dir/bin/log.html" 2>/dev/null | grep -i -E 'fps|score|phoronix' | tail -20 >"$out/bench.txt" || true
		;;
	esac
	summary "$tag"
}

# The same GPU load as Direct3D 11 through Macron's DXMT (gpuload.exe) and
# as native Metal under Rosetta and on arm64 (gpuload), each measured by the
# Metal HUD: the difference in GPU time a frame is DXMT's.
pair() {
	tag=$1 size=${2:-1600x900}
	[ -f "$home/gpuload.exe" ] && [ -x "$home/gpuload" ] || die "put gpuload.exe and gpuload into $home first (docs/perf/real-mac-requests.md)"
	for load in fill draws mix; do
		for api in d3d11 metal-x86_64 metal-arm64; do
			out="$home/results/$tag-$load-$api"
			[ -e "$out" ] && die "$out exists; pick another tag"
			mkdir -p "$out"
			{
				echo "gpuload $load, $api, $size, $(date -u +%Y-%m-%dT%H:%M:%SZ)"
				echo "macOS $(sw_vers -productVersion), $(sysctl -n machdep.cpu.brand_string)"
				pmset -g batt | sed -n 1p
				echo "waited $(wait_cool) s for thermal pressure 0 before starting"
			} >"$out/env.txt"
			thermal_logger >"$out/thermal.txt" &
			thermal=$!
			(
				sleep 300
				echo "timed out after 300 s: ended gpuload" >>"$out/env.txt"
				pkill -f gpuload
			) &
			watchdog=$!
			set -- --mode "$load" --size "$size" --seconds 20
			case $api in
			d3d11)
				env MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1 SteamAppId=990004 STEAM_COMPAT_DATA_PATH="$home/gpuload-prefix" \
					macron waitforexitandrun "$home/gpuload.exe" "$@" --out "Z:$out/bench.txt" >"$out/macron.log" 2>&1 || true
				;;
			metal-x86_64)
				env MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1 arch -x86_64 "$home/gpuload" "$@" --out "$out/bench.txt" >"$out/macron.log" 2>&1 || true
				;;
			metal-arm64)
				env MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1 arch -arm64 "$home/gpuload" "$@" --out "$out/bench.txt" >"$out/macron.log" 2>&1 || true
				;;
			esac
			kill $thermal $watchdog 2>/dev/null || true
			summary "$tag-$load-$api" >/dev/null
		done
	done
	for out in "$home/results/$tag"-*; do
		printf '%s: %s; %s\n' "${out##*/}" "$(grep -m1 'frames a second' "$out/summary.txt")" "$(grep -m1 'GPU time ms' "$out/summary.txt")"
	done | tee "$home/results/$tag-table.txt"
}

# The HUD's line is "metal-HUD: <frame>,<memory>,<memory>[,<misses>]" and
# then "<interval ms>,<GPU ms>" for each frame since the previous line.
summary() {
	out="$home/results/$1"
	[ -d "$out" ] || die "no results for '$1'"
	awk '
		/metal-HUD:/ {
			line = substr($0, index($0, "metal-HUD:") + 10)
			n = split(line, f, ",")
			for (i = 1; i <= n; i++) gsub(/[^0-9.]/, "", f[i])
			first = f[1]
			rest = n - 3
			start = 4
			if (rest % 2 == 1) start = 5
			for (i = start; i + 1 <= n; i += 2) print first++ "," f[i] "," f[i + 1]
		}' "$out/macron.log" >"$out/hud.csv"
	frames=$(wc -l <"$out/hud.csv" | tr -d ' ')
	{
		cat "$out/env.txt"
		echo
		echo "HUD frames logged: $frames"
		if [ "$frames" -gt 100 ]; then
			# Leave out the first 10% (shader compiles, the scene coming up).
			skip=$((frames / 10))
			tail -n +$((skip + 1)) "$out/hud.csv" | awk -F, '
				{ n++; si += $2; sg += $3; iv[n] = $2; gp[n] = $3 }
				END {
					printf "frames measured: %d (the first %d left out)\n", n, '"$skip"'
					printf "frames a second: %.1f\n", 1000 * n / si
					printf "GPU busy: %.0f%% (sum of GPU time over sum of intervals)\n", 100 * sg / si
				}'
			for col in 2 3; do
				name=interval
				[ $col = 3 ] && name="GPU time"
				tail -n +$((skip + 1)) "$out/hud.csv" | cut -d, -f$col | sort -n | awk -v name="$name" '
					{ v[++n] = $1 }
					END { printf "%s ms: p25 %.2f, median %.2f, p75 %.2f, p95 %.2f, p99 %.2f\n", name, v[int(n*.25)+1], v[int(n*.5)+1], v[int(n*.75)+1], v[int(n*.95)+1], v[int(n*.99)+1] }'
			done
		else
			echo "too few HUD lines to summarise; the first raw lines:"
			grep -m 3 'metal-HUD' "$out/macron.log" | cut -c1-300 || echo "(none)"
		fi
		echo "thermal pressure levels seen: $(awk '{print $2}' "$out/thermal.txt" 2>/dev/null | sort | uniq -c | tr '\n' ' ')"
		echo
		echo "benchmark's own result:"
		cat "$out/bench.txt" 2>/dev/null || true
		echo
		echo "DXMT and Metal lines:"
		grep -E '^(err|warn):|MetalFX|GPU Hang|Device error|LAUNCH_FAILED|exited with' "$out/macron.log" |
			sed -e 's/frame [0-9]*/frame N/' -e 's/0x[0-9a-f]*/0x…/g' | cut -c1-200 | sort | uniq -c | sort -rn | head -15
	} >"$out/summary.txt"
	cat "$out/summary.txt"
}

case ${1:-} in
setup) [ $# -eq 2 ] || die "usage: bench.sh setup <bench>" && setup "$2" ;;
run) [ $# -ge 3 ] || die "usage: bench.sh run <bench> <tag> [WxH] [windowed|fullscreen] [NAME=VALUE ...]" && shift && run "$@" ;;
pair) [ $# -ge 2 ] || die "usage: bench.sh pair <tag> [WIDTHxHEIGHT]" && shift && pair "$@" ;;
summary) [ $# -eq 2 ] || die "usage: bench.sh summary <tag>" && summary "$2" ;;
*) die "usage: bench.sh setup|run|summary|pair ..." ;;
esac
