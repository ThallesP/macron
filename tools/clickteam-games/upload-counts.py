"""upload-counts.py <run-dir> <n>: what Ultimate Custom Night's buffer uploads
look like to the host OpenGL, from a ucn-run.sh start with
WINEDEBUG=-all,+opengl. Per place (title, menu, night), over the measured
window only: draws, glBufferSubData, synchronized and unsynchronized
glMapBufferRange, and glBufferWriteUnsynchronizedWINE (Wine patch 0009).
On Apple's OpenGL over Metal the first two wait for the GPU whenever issued
draws still use the buffer (docs/proof/gate1-m3/)."""
import collections
import re
import sys

CALL = re.compile(r"trace:opengl:(\w+) ")
ACCESS = re.compile(r"access (\d+)")


def kind(name, line):
    if name.startswith(("glDraw", "glMultiDraw")):
        return "draws"
    if name in ("glBufferSubData", "glBufferSubDataARB", "glNamedBufferSubData"):
        return "glBufferSubData"
    if name == "glBufferWriteUnsynchronizedWINE":
        return "unsynchronized writes"
    if name in ("glMapBufferRange", "glMapNamedBufferRange"):
        access = ACCESS.search(line)
        return "unsynchronized maps" if access and int(access.group(1)) & 0x20 else "synchronized maps"
    if name in ("glMapBuffer", "glMapBufferARB", "glMapNamedBuffer"):
        return "synchronized maps"
    return None


def main(run_dir, n):
    marks = {}
    for line in open(f"{run_dir}/{n}.phases"):
        name, at = line.split()
        marks[name] = float(at)
    places = [p for p in ("title", "menu", "night") if f"{p}-start" in marks and f"{p}-end" in marks]
    counts = {p: collections.Counter() for p in places}
    for line in open(f"{run_dir}/{n}.log", errors="replace"):
        stamp, _, rest = line.partition(" ")
        try:
            at = float(stamp)
        except ValueError:
            continue
        match = CALL.search(rest)
        if not match or not (what := kind(match.group(1), rest)):
            continue
        for place in places:
            if marks[f"{place}-start"] <= at < marks[f"{place}-end"]:
                counts[place][what] += 1
    kinds = ["draws", "glBufferSubData", "synchronized maps", "unsynchronized writes", "unsynchronized maps"]
    print("| place | " + " | ".join(kinds) + " |")
    print("|---" * (len(kinds) + 1) + "|")
    for place in places:
        print(f"| {place} | " + " | ".join(str(counts[place][k]) for k in kinds) + " |")


main(sys.argv[1], sys.argv[2])
