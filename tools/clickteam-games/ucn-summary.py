"""ucn-summary.py: reads what ucn-run.sh records.

  result <run-dir> <n> <state>   one JSON line for start n (ucn-run.sh calls it)
  table <run-dir>...             one line per place and start, every start listed

Per place (title, menu, night), over the measured window only:
  fps      wined3d's frame counter: the median of its samples (one about every
           1.5 s), the lowest and highest, and the mean (frames counted from the
           samples, divided by the time they cover)
  count    lines matching UCN_COUNT (a regex, e.g. a +bitblt call that the game
           makes once per frame), per second
  cpu      CPU time of each process over the window, % of one core: the game,
           wineserver, the rest of Wine, and the busiest other processes
"""
import json, os, re, statistics, sys

PLACES = ["title", "menu", "night"]


def read_phases(path):
    marks = {}
    for line in open(path):
        name, t = line.split()
        marks[name] = float(t)
    return marks


def read_ps(path):
    procs = {}
    for line in open(path, errors="replace"):
        m = re.match(r"\s*(\d+)\s+(\S+)\s+(.*)", line)
        if not m:
            continue
        secs = 0.0
        for part in m.group(2).split(":"):
            secs = secs * 60 + float(part)
        procs[int(m.group(1))] = (secs, m.group(3).strip())
    return procs


def kind(args):
    if "Ultimate Custom Night.exe" in args and "waitforexitandrun" not in args:
        return "game"
    if "wineserver" in args:
        return "wineserver"
    if re.match(r"[A-Za-z]:\\", args) or "Macron/runtimes" in args:
        return "wine-other"
    return None


def cpu(d, n, place, seconds):
    a, b = os.path.join(d, f"{n}.ps-{place}-0"), os.path.join(d, f"{n}.ps-{place}-1")
    if not (os.path.exists(a) and os.path.exists(b)) or seconds <= 0:
        return None
    p0, p1 = read_ps(a), read_ps(b)
    out, others = {"game": 0.0, "wineserver": 0.0, "wine-other": 0.0}, []
    for pid, (t1, args) in p1.items():
        if pid not in p0:
            continue
        pct = 100 * (t1 - p0[pid][0]) / seconds
        k = kind(args)
        if k:
            out[k] += pct
        elif pct >= 3:
            others.append((round(pct, 1), os.path.basename(args.split()[0])[:40]))
    r = {k: round(v, 1) for k, v in out.items()}
    r["others"] = sorted(others, reverse=True)[:4]
    return r


def result(d, n, state):
    marks = read_phases(os.path.join(d, f"{n}.phases"))
    count_re = re.compile(os.environ["UCN_COUNT"]) if os.environ.get("UCN_COUNT") else None
    lines, crash = [], None
    for line in open(os.path.join(d, f"{n}.log"), errors="replace"):
        ts, _, rest = line.partition(" ")
        try:
            ts = float(ts)
        except ValueError:
            continue
        if crash is None and ("Unhandled" in rest or "unhandled exception" in rest):
            crash = rest.strip()[:200]
        lines.append((ts, rest))
    fps_all = [(t, float(m.group(1))) for t, l in lines
               if (m := re.search(r"wined3d_cs_exec_present .*@ approx ([0-9.]+)fps", l))]
    r = {"run": int(n), "state": state, "crash": crash,
         "windowAfterS": round(marks["window"] - marks["launch"], 1) if "window" in marks else None}
    for place in PLACES:
        s, e = marks.get(f"{place}-start"), marks.get(f"{place}-end")
        if s is None or e is None:
            continue
        p = {"seconds": round(e - s, 1)}
        idx = [i for i, (t, _) in enumerate(fps_all) if s <= t <= e]
        if idx:
            vals = [fps_all[i][1] for i in idx]
            p["fpsMedian"] = round(statistics.median(vals), 2)
            p["fpsMin"], p["fpsMax"], p["samples"] = min(vals), max(vals), len(vals)
            spans = [(fps_all[i][1], fps_all[i][0] - fps_all[i - 1][0]) for i in idx if i > 0]
            if spans:
                p["fpsMean"] = round(sum(f * dt for f, dt in spans) / sum(dt for _, dt in spans), 2)
        if count_re:
            hits = sum(1 for t, l in lines if s <= t <= e and count_re.search(l))
            p["countPerS"] = round(hits / (e - s), 2)
        p["cpu"] = cpu(d, n, place, e - s)
        r[place] = p
    print(json.dumps(r))


def table(dirs):
    for d in dirs:
        label = os.path.basename(d.rstrip("/"))
        path = os.path.join(d, "results.jsonl")
        if not os.path.exists(path):
            continue
        cfg = open(os.path.join(d, "1.config")).read().strip() if os.path.exists(os.path.join(d, "1.config")) else ""
        print(f"== {label}: {cfg}")
        for line in open(path):
            r = json.loads(line)
            head = f"   start {r['run']}: {r['state']}, window after {r['windowAfterS']} s"
            if r.get("crash"):
                head += f", crash: {r['crash'][:120]}"
            print(head)
            for place in PLACES:
                p = r.get(place)
                if not p:
                    continue
                parts = [f"{place:5}"]
                if "fpsMedian" in p:
                    parts.append(f"fps median {p['fpsMedian']} mean {p.get('fpsMean')} ({p['fpsMin']}..{p['fpsMax']}, {p['samples']} samples)")
                if "countPerS" in p:
                    parts.append(f"count {p['countPerS']}/s")
                if p.get("cpu"):
                    c = p["cpu"]
                    parts.append(f"CPU game {c['game']}% wineserver {c['wineserver']}% wine-other {c['wine-other']}% others {c['others']}")
                print("     " + ", ".join(parts))


if __name__ == "__main__":
    if sys.argv[1] == "result":
        result(*sys.argv[2:5])
    else:
        table(sys.argv[2:])
