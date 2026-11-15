#!/bin/bash
# copies.sh <label> <max-starts> <warmup-s> <trace-s> <game-id> <exe> <title-regex>
# With WINEDEBUG=-all,+timestamp,+opengl,+fps: start the game from a fresh
# clone of the prepared prefix until one start reaches its window and lives
# through warm-up and trace; keep only the copy lines of opengl32's wow64
# buffer maps ("Copying 0x...") and the frame counter, and count copies per
# presented frame between the first and the last frame-counter line.
set -u
label=$1 max=$2 warm=$3 secs=$4 gid=$5 exe=$6 title=$7
ct=$HOME/ct
macron=$ct/bin/macron
out=$ct/copies/$label
mkdir -p "$out"
prefix="$HOME/Library/Application Support/Macron/prefixes/$gid"
kill_wine() {
  "$macron" kill --game "$gid" >/dev/null 2>&1
  pkill -9 -f 'Macron/runtimes' 2>/dev/null
  pkill -9 -f 'ct/bin/macron launch' 2>/dev/null
  pkill -9 -f '^[A-Za-z]:\\' 2>/dev/null
  sleep 2
}
for run in $(seq 1 "$max"); do
  kill_wine
  rm -rf "$prefix"
  cp -cR "$ct/prefix-ready/$gid" "$prefix"
  log=$out/$run.log
  start=$(date +%s)
  WINEDEBUG=-all,+timestamp,+opengl,+fps "$macron" launch --game "$gid" --cwd "$(dirname "$exe")" -- "$exe" \
    > >(grep -a --line-buffered -E 'Copying 0x|trace:fps|Unhandled|unhandled|exited with' >"$log") 2>&1 &
  pid=$!
  wid="" state=""
  while [ -z "$wid" ]; do
    sleep 1
    kill -0 "$pid" 2>/dev/null || { state=exit-before-window; break; }
    grep -aq 'nhandled' "$log" 2>/dev/null && { sleep 2; state=crash-before-window; break; }
    [ $(($(date +%s) - start)) -ge 300 ] && { state=no-window; break; }
    wid=$("$ct/scripts/macwin" windows | python3 -I -c '
import json,re,sys
w=[x for x in json.load(sys.stdin) if x["owner"].lower().startswith("wine") and re.search(sys.argv[1], x["title"]) and x["width"]>100]
print(w[0]["id"] if w else "")' "$title")
  done
  if [ -z "$state" ]; then
    sleep $((warm + secs))
    kill -0 "$pid" 2>/dev/null || state=died
  fi
  kill_wine
  kill -9 "$pid" 2>/dev/null
  wait "$pid" 2>/dev/null
  sleep 1
  python3 -I - "$label" "$run" "${state:-ok}" "$warm" "$log" >>"$out/results.jsonl" <<'EOF'
import json,re,sys
label,run,state,warm,log=sys.argv[1:]
lines=[]
for line in open(log,errors="replace"):
    m=re.match(r"\s*([0-9]+\.[0-9]+):",line)
    if m: lines.append((float(m.group(1)),line))
crash=next((l.strip()[:200] for l in open(log,errors="replace") if "nhandled" in l),None)
fps=[(t,float(re.search(r"approx ([0-9.]+)fps",l).group(1))) for t,l in lines if "wined3d_cs_exec_present" in l and "approx" in l]
r={"label":label,"run":int(run),"state":"crash" if crash and state=="ok" else state,"crash":crash}
if fps:
    first=fps[0][0]+float(warm)
    win=[(t,f) for t,f in fps if t>=first]
    if len(win)>=2:
        a,b=win[0][0],win[-1][0]
        frames=0.0;prev=a
        for t,f in win[1:]:
            frames+=f*(t-prev); prev=t
        copies=[l for t,l in lines if a<t<=b and "Copying 0x" in l]
        size=sum(int(re.search(r"Copying (0x[0-9a-fA-F]+)",l).group(1),16) for l in copies)
        r.update({"seconds":round(b-a,2),"frames":round(frames,1),"copies":len(copies),"bytes":size,
                  "copiesPerFrame":round(len(copies)/frames,1) if frames else None,
                  "bytesPerFrame":round(size/frames) if frames else None,
                  "fpsWhileTracing":[f for _,f in win]})
print(json.dumps(r))
EOF
  tail -1 "$out/results.jsonl"
  tail -1 "$out/results.jsonl" | grep -q '"state": "ok"' && break
done
echo done >"$out/done"
