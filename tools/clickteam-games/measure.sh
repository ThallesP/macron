#!/bin/bash
# measure.sh <label> <ok-runs> <max-starts> <warmup-s> <measure-s> <game-id> <exe> <title-regex>
# Starts the game through `macron launch` until <ok-runs> starts were measured
# without a crash or stall, at most <max-starts> times, each from a fresh APFS
# clone of ~/ct/prefix-ready/<game-id>, every Wine process killed before and
# after. Waits for the game's window, <warmup-s> more, then reads wined3d's
# frame counter (WINEDEBUG=+fps lines, timestamped as they arrive) and the CPU
# time of the game process and wineserver over <measure-s> seconds, and
# screenshots the window. Results: ~/ct/meas/<label>/results.jsonl.
# Environment: WINEDEBUG, MACRON_OPENGL, WINE_D3D_CONFIG ... pass through.
# Layout on the Mac (shared by the scripts in this folder): ~/ct/bin/macron (a
# darwin/arm64 build), ~/ct/scripts/ holding these scripts and macwin (built
# from internal/check/macwin.swift with `xcrun swiftc -O`), the prepared
# prefix from prepare.sh in ~/ct/prefix-ready/, and for gltrace.sh the two
# wined3d.dll builds in ~/ct/stock/{m4,stock}/{i386,x64}/. PROGRESS.md, "A real
# Clickteam Fusion game", has the runs.
set -u
label=$1 want=$2 max=$3 warm=$4 meas=$5 gid=$6 exe=$7 title=$8
ct=$HOME/ct
macron=$ct/bin/macron
out=$ct/meas/$label
mkdir -p "$out"
prefix="$HOME/Library/Application Support/Macron/prefixes/$gid"

kill_wine() {
  "$macron" kill --game "$gid" >/dev/null 2>&1
  pkill -9 -f 'Macron/runtimes' 2>/dev/null
  pkill -9 -f 'ct/bin/macron launch' 2>/dev/null
  pkill -9 -f '^[A-Za-z]:\\' 2>/dev/null
  sleep 2
}

cputime() { # seconds of CPU used by a pid, or empty
  ps -o time= -p "$1" 2>/dev/null | python3 -I -c '
import sys
t=sys.stdin.read().strip()
if t:
    s=0.0
    for p in t.split(":"): s=s*60+float(p)
    print(s)'
}

oks=0
for run in $(seq 1 "$max"); do
  [ "$oks" -ge "$want" ] && break
  t0=0 t1=0 c0="" c1="" w0="" w1=""
  kill_wine
  rm -rf "$prefix"
  cp -cR "$ct/prefix-ready/$gid" "$prefix"
  log=$out/$run.log
  start=$(date +%s)
  "$macron" launch --game "$gid" --cwd "$(dirname "$exe")" -- "$exe" \
    > >(perl -MTime::HiRes=time -ne 'BEGIN{$|=1} printf "%.3f %s", time, $_' >"$log") 2>&1 &
  pid=$!
  state="" gpid="" wid=""
  while [ -z "$wid" ]; do
    sleep 1
    if ! kill -0 "$pid" 2>/dev/null; then state=exit-before-window; break; fi
    grep -aq 'nhandled' "$log" 2>/dev/null && { sleep 2; state=crash-before-window; break; }
    if [ $(($(date +%s) - start)) -ge 240 ]; then state=no-window; break; fi
    read -r wid gpid < <("$ct/scripts/macwin" windows | python3 -I -c '
import json,re,sys
w=[x for x in json.load(sys.stdin) if x["owner"].lower().startswith("wine") and re.search(sys.argv[1], x["title"]) and x["width"]>100]
print(w[0]["id"], w[0]["pid"]) if w else print("")' "$title")
  done
  twin=$(date +%s)
  if [ -z "$state" ]; then
    sleep "$warm"
    ws=$(pgrep -f 'Macron/runtimes/.*wineserver' | head -1)
    screencapture -x -o -l "$wid" "$out/$run-window-start.png" 2>/dev/null
    t0=$(python3 -I -c 'import time;print(time.time())')
    c0=$(cputime "$gpid") w0=$(cputime "$ws")
    for i in $(seq 1 "$meas"); do
      sleep 1
      kill -0 "$gpid" 2>/dev/null || { state=died-while-measuring; break; }
    done
    t1=$(python3 -I -c 'import time;print(time.time())')
    c1=$(cputime "$gpid") w1=$(cputime "$ws")
    screencapture -x -o -l "$wid" "$out/$run-window.png" 2>/dev/null
    "$ct/scripts/macwin" windows >"$out/$run-windows.json"
  fi
  kill_wine
  kill -9 "$pid" 2>/dev/null
  wait "$pid" 2>/dev/null
  sleep 1
  python3 -I - "$label" "$run" "${state:-ok}" "${t0:-0}" "${t1:-0}" "${c0:-}" "${c1:-}" "${w0:-}" "${w1:-}" "$((twin - start))" "$log" >>"$out/results.jsonl" <<'EOF'
import json,re,statistics,sys
label,run,state,t0,t1,c0,c1,w0,w1,twin,log=sys.argv[1:]
t0,t1=float(t0),float(t1)
fps=[];crash=None;total=0;last=0
for line in open(log,errors="replace"):
    ts,_,rest=line.partition(" ")
    try: ts=float(ts)
    except ValueError: continue
    if crash is None and ("Unhandled" in rest or "unhandled exception" in rest): crash=rest.strip()[:200]
    m=re.search(r"wined3d_cs_exec_present .*@ approx ([0-9.]+)fps",rest)
    if m:
        total+=1
        if t0<=ts<=t1: fps.append(float(m.group(1))); last=ts
if crash and state=="ok": state="crash"
if state=="ok" and (not fps or last<t1-10): state="stall"
dur=t1-t0
def pct(a,b): return round(100*(float(b)-float(a))/dur,1) if a and b and dur>0 else None
r={"label":label,"run":int(run),"state":state,"windowAfterS":int(twin),"samples":len(fps),
   "fpsMedian":round(statistics.median(fps),2) if fps else None,"fpsMin":min(fps) if fps else None,"fpsMax":max(fps) if fps else None,
   "cpuGamePct":pct(c0,c1),"cpuWineserverPct":pct(w0,w1),"fpsLinesTotal":total,"crash":crash}
print(json.dumps(r))
EOF
  tail -1 "$out/results.jsonl"
  tail -1 "$out/results.jsonl" | grep -q '"state": "ok"' && oks=$((oks + 1))
done
echo done >"$out/done"
