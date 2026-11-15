"""ucn-glcount.py <run-dir> <n>: OpenGL calls per presented frame at each place
(title, menu, night) of one ucn-run.sh start traced with
WINEDEBUG=-all,+opengl,+fps. Frames are counted from wined3d's frame counter
lines inside the measured window (samples times the time each covers); calls
are opengl32's trace lines between the first and the last of those samples.
Also the bytes per frame that pass through glBufferSubData/glBufferData (size),
glTexSubImage2D/glTexImage2D (width x height x 4) and opengl32's wow64 buffer
map copies ("Copying 0x... bytes").
"""
import collections, re, sys

d, n = sys.argv[1], sys.argv[2]
marks = {}
for line in open(f"{d}/{n}.phases"):
    k, t = line.split()
    marks[k] = float(t)

LINE = re.compile(r"(\d+\.\d+) [0-9a-f]+:trace:opengl:(\w+) ?(.*)")
FPS = re.compile(r"(\d+\.\d+) [0-9a-f]+:trace:fps:wined3d_cs_exec_present .*@ approx ([0-9.]+)fps")
fps, calls = [], []
for line in open(f"{d}/{n}.log", errors="replace"):
    if m := FPS.match(line):
        fps.append((float(m.group(1)), float(m.group(2))))
    elif m := LINE.match(line):
        calls.append((float(m.group(1)), m.group(2), m.group(3)))


def nbytes(fn, args):
    if fn in ("glBufferSubData", "glBufferData", "glNamedBufferSubData"):
        m = re.search(r"size (\d+)", args)
        return int(m.group(1)) if m else 0
    if fn in ("glTexSubImage2D", "glTexImage2D"):
        m = re.search(r"width (\d+), height (\d+)", args)
        return int(m.group(1)) * int(m.group(2)) * 4 if m else 0
    if "Copying" in args or fn.startswith("wow64"):
        m = re.search(r"Copying (0x[0-9a-fA-F]+|\d+)", args)
        return int(m.group(1), 0) if m else 0
    return 0


for place in ("title", "menu", "night"):
    s, e = marks.get(f"{place}-start"), marks.get(f"{place}-end")
    if s is None:
        continue
    win = [(t, f) for t, f in fps if s <= t <= e]
    if len(win) < 2:
        print(f"== {place}: fewer than 2 frame-counter samples")
        continue
    a, b = win[0][0], win[-1][0]
    frames = sum(f * (t - p) for (p, _), (t, f) in zip(win, win[1:]))
    count, size = collections.Counter(), collections.Counter()
    for t, fn, args in calls:
        if a < t <= b:
            key = "Copying (wow64 map)" if "Copying" in args else fn
            count[key] += 1
            size[key] += nbytes(fn, args)
    print(f"== {place}: {b - a:.1f} s, {frames:.0f} frames ({frames / (b - a):.1f} fps while traced), {sum(count.values())} opengl trace lines")
    for fn, c in count.most_common(30):
        extra = f", {size[fn] / frames / 1024:.0f} KiB/frame" if size[fn] else ""
        print(f"   {fn}: {c / frames:.1f}/frame{extra}")
