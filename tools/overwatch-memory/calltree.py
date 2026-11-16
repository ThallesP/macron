#!/usr/bin/env python3
"""calltree.py <malloc_history -callTree output> [min_mb] [depth]

Prints the call tree's nodes that hold at least min_mb (default 100) of live
malloc, down to `depth` levels (default 40), one line per node: depth, count,
size, frame. Frames with one child of the same size are folded into a chain
so that long single paths stay readable.
"""
import re
import sys

NODE = re.compile(r"^(?P<lead>[ +!:|]*)(?P<count>\d+) \((?P<size>[\d.]+)(?P<unit>[KMG]?)\) (?P<frame>.*)$")
UNIT = {"": 1 / 1048576, "K": 1 / 1024, "M": 1, "G": 1024}


def parse(path):
    root = {"frame": "TOTAL", "mb": 0, "count": 0, "kids": [], "depth": -1}
    stack = [root]
    started = False
    with open(path, errors="replace") as f:
        for line in f:
            line = line.rstrip("\n")
            if "<< TOTAL >>" in line:
                started = True
                m = NODE.match(line.strip())
                if m:
                    root["mb"] = float(m["size"]) * UNIT[m["unit"]]
                    root["count"] = int(m["count"])
                continue
            if not started:
                continue
            m = NODE.match(line)
            if not m:
                if line.strip().startswith("0x"):
                    break
                continue
            depth = len(m["lead"])
            node = {
                "frame": shorten(m["frame"]),
                "mb": float(m["size"]) * UNIT[m["unit"]],
                "count": int(m["count"]),
                "kids": [],
                "depth": depth,
            }
            while stack[-1]["depth"] >= depth:
                stack.pop()
            stack[-1]["kids"].append(node)
            stack.append(node)
    return root


def shorten(frame):
    frame = re.sub(r"\s+\[0x[0-9a-f]+\]$", "", frame)
    frame = re.sub(r"load address 0x[0-9a-f]+ \+ ", "", frame)
    frame = re.sub(r"std::__variant_detail::__visitation::__base::__dispatcher<(\d+)ul>::__dispatch\S*", r"variant-dispatch<\1>", frame)
    return frame[:200]


def show(node, min_mb, depth, level=0):
    if level > depth:
        return
    for kid in sorted(node["kids"], key=lambda k: -k["mb"]):
        if kid["mb"] < min_mb:
            continue
        chain = [kid]
        while len(chain[-1]["kids"]) == 1 and abs(chain[-1]["kids"][0]["mb"] - chain[-1]["mb"]) < 0.5:
            chain.append(chain[-1]["kids"][0])
        frames = " > ".join(c["frame"] for c in chain)
        print(f"{'  ' * level}{kid['count']:>8} {kid['mb']:>8.0f}M  {frames}")
        show(chain[-1], min_mb, depth, level + 1)


if __name__ == "__main__":
    tree = parse(sys.argv[1])
    min_mb = float(sys.argv[2]) if len(sys.argv) > 2 else 100
    depth = int(sys.argv[3]) if len(sys.argv) > 3 else 40
    print(f"{tree['count']:>8} {tree['mb']:>8.0f}M  TOTAL")
    show(tree, min_mb, depth)
