"""summary.py <meas-dir> <label>...: one line per configuration from measure.sh's
results (several labels can be pooled, e.g. a and a2) plus exit codes from the logs."""
import json, os, re, statistics, sys
from collections import Counter

root = sys.argv[1]
groups = {}
for arg in sys.argv[2:]:
    name, labels = arg.split("=")
    groups[name] = labels.split(",")

for name, labels in groups.items():
    rows, exits, faults = [], Counter(), Counter()
    for label in labels:
        d = os.path.join(root, label)
        for line in open(os.path.join(d, "results.jsonl")):
            r = json.loads(line)
            rows.append(r)
            if r["state"] != "ok":
                log = open(os.path.join(d, f"{r['run']}.log"), errors="replace").read()
                m = re.findall(r"exited with (\d+)", log)
                exits[m[-1] if m else "?"] += 1
                f = re.search(r"at address ([0-9A-F]+)", r["crash"] or "")
                faults[f.group(1) if f else "no fault line"] += 1
    ok = [r for r in rows if r["state"] == "ok"]
    samples = []
    print(f"== {name}: {len(rows)} starts, {len(ok)} measured, states {dict(Counter(r['state'] for r in rows))}")
    print(f"   failed starts: exit codes {dict(exits)}, faults {dict(faults)}")
    for r in ok:
        print(f"   run {r['label']}/{r['run']}: fps median {r['fpsMedian']} min {r['fpsMin']} max {r['fpsMax']} ({r['samples']} samples), CPU game {r['cpuGamePct']}% wineserver {r['cpuWineserverPct']}%")
    if ok:
        med = [r["fpsMedian"] for r in ok]
        print(f"   over runs: median of run medians {statistics.median(med)}, lowest sample {min(r['fpsMin'] for r in ok)}, highest {max(r['fpsMax'] for r in ok)}, CPU {[r['cpuGamePct'] for r in ok]}")
