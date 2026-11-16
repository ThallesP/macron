"""sample-paths.py <sample file> <thread>: where one thread of a macOS `sample`
report spends its time, by the libraries on each sample's stack (Apple's
software OpenGL renderer, Apple's OpenGL front end, Wine's opengl32, Rosetta,
...). Each sample counts once, for the innermost frame it was taken in."""
import collections
import re
import sys

PATTERN = re.compile(r"^(\s*[+!:| ]*)(\d+) (.*)$")


def library(text):
    match = re.search(r"\(in ([^)]+)\)", text)
    return match.group(1) if match else ""


def classify(libraries):
    present = set(libraries)
    if present & {"GLRendererFloat", "libGLImage.dylib", "libGLProgrammability.dylib"}:
        return "Apple software renderer (raster, shaders)"
    if present & {"GLEngine", "libGL.dylib"}:
        return "Apple OpenGL front end"
    for name, label in (("opengl32.so", "Wine opengl32 unix side"), ("winemac.so", "winemac"), ("win32u.so", "win32u")):
        if name in present:
            return label
    if any("Rosetta Runtime" in l for l in libraries):
        return "Rosetta runtime routines (outside GL)"
    if any("Rosetta JIT" in l for l in libraries):
        return "Rosetta JIT code (outside GL)"
    if "ntdll.so" in present:
        return "ntdll unix side"
    return "other: " + (libraries[-1] if libraries else "?")


def main(path, thread):
    rows, stack, current = [], [], None
    for line in open(path, errors="replace"):
        match = PATTERN.match(line.rstrip("\n"))
        if not match:
            continue
        depth, count, text = len(match.group(1)), int(match.group(2)), match.group(3)
        if text.startswith("Thread_"):
            current, stack = text.split()[0], []
            continue
        if current != thread:
            continue
        while stack and stack[-1][0] >= depth:
            stack.pop()
        stack.append((depth, library(text)))
        rows.append((depth, count, [l for _, l in stack]))

    totals = collections.Counter()
    for i, (depth, count, libraries) in enumerate(rows):
        child_depth, children = None, 0
        for later_depth, later_count, _ in rows[i + 1:]:
            if later_depth <= depth:
                break
            child_depth = child_depth or later_depth
            if later_depth == child_depth:
                children += later_count
        if count > children:
            totals[classify(libraries)] += count - children

    total = sum(totals.values())
    print(path, thread, "samples", total)
    for label, count in totals.most_common():
        print(f"  {100 * count / total:5.1f}%  {label}")


main(sys.argv[1], sys.argv[2])
