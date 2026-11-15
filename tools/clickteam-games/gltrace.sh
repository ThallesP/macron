#!/bin/bash
# gltrace.sh <label> <wined3d: m4|stock> <exe> <title-regex>: one start with
# WINEDEBUG=-all,+timestamp,+opengl,+fps and nothing filtered, 10 s after the
# window, then the number of each traced OpenGL call per presented frame over
# the last 5 s (which calls the game's frames make, and whether any maps a buffer).
set -u
label=$1 which=$2 exe=$3 title=$4
ct=$HOME/ct
M="$HOME/Library/Application Support/Macron"
R=$(ls -d "$M"/runtimes/*/ | head -1)wine-dxmt/lib/wine
out=$ct/gltrace/$label
mkdir -p "$out"
cp "$ct/stock/$which/i386/wined3d.dll" "$R/i386-windows/wined3d.dll"
cp "$ct/stock/$which/x64/wined3d.dll" "$R/x86_64-windows/wined3d.dll"
shasum -a 256 "$R"/*-windows/wined3d.dll >"$out/wined3d.txt"
pkill -9 -f 'Macron/runtimes'; sleep 1
rm -rf "$M/prefixes/tomaze"; cp -cR "$ct/prefix-ready/tomaze" "$M/prefixes/tomaze"
MACRON_OPENGL=apple-software WINEDEBUG=-all,+timestamp,+opengl,+fps "$ct/bin/macron" launch --game tomaze --cwd "$(dirname "$exe")" -- "$exe" >"$out/trace.log" 2>&1 &
for i in $(seq 1 120); do
  sleep 1
  "$ct/scripts/macwin" windows | grep -q "\"title\":\"${title#^}" && break
done
sleep 10
pkill -9 -f 'Macron/runtimes'; pkill -9 -f 'ct/bin/macron launch'; pkill -9 -f '^[A-Za-z]:\\'
sleep 1
cp "$ct/stock/m4/i386/wined3d.dll" "$R/i386-windows/wined3d.dll"
cp "$ct/stock/m4/x64/wined3d.dll" "$R/x86_64-windows/wined3d.dll"
python3 -I - "$out/trace.log" >"$out/calls.txt" <<'EOF'
import collections,re,sys
lines=[]
for l in open(sys.argv[1],errors="replace"):
    m=re.match(r"\s*([0-9]+\.[0-9]+):[0-9a-f]+:trace:(\w+):(\w+)",l)
    if m: lines.append((float(m.group(1)),m.group(2),m.group(3),l))
end=lines[-1][0]; start=end-5
fps=[(t,float(re.search(r"approx ([0-9.]+)fps",l).group(1))) for t,c,f,l in lines if f=="wined3d_cs_exec_present" and "approx" in l]
win=[(t,f) for t,f in fps if t>=start]
frames=sum(f*(t-p) for (p,_),(t,f) in zip(win,win[1:]))
a,b=win[0][0],win[-1][0]
calls=collections.Counter(f for t,c,f,l in lines if c=="opengl" and a<t<=b)
copies=sum(1 for t,c,f,l in lines if a<t<=b and "Copying 0x" in l)
print(f"window {a:.2f}..{b:.2f} s, frames {frames:.1f}, opengl trace lines {sum(calls.values())}, Copying lines {copies}")
for f,n in calls.most_common(40): print(f"{f} {n} {n/frames:.1f}/frame")
EOF
ls -la "$out/trace.log" >>"$out/calls.txt"
echo done >"$out/done"
