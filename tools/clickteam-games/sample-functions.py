"""sample-functions.py <sample> <thread> <bases> <dll-dir> [top]: where one thread
of a macOS `sample` report of a 32-bit Windows program under Wine spends its
time, by function. x86 frames are resolved with llvm-symbolizer against the
DLLs in <dll-dir> (Macron's overlay DLLs carry DWARF), placed by <bases>
("<hex load address> <dll name>" lines, from a WINEDEBUG=+loaddll log);
native frames keep the name `sample` gives them.

Prints self time (the innermost frame of each sample) and inclusive time
(any frame of the sample) per function, as shares of the thread's samples.
LLVM_SYMBOLIZER and LLVM_OBJDUMP name the tools (default: on PATH)."""
import collections
import os
import re
import subprocess
import sys

PATTERN = re.compile(r"^(\s*[+!:| ]*)(\d+) (.*)$")
ADDRESS = re.compile(r"\[0x([0-9a-f]+)\]")
NAMED = re.compile(r"^(\S+.*?)\s+\(in ([^)]+)\)")


def image_info(path):
    out = subprocess.run([os.environ.get("LLVM_OBJDUMP", "llvm-objdump"), "-p", path],
                         capture_output=True, text=True).stdout
    base = int(re.search(r"ImageBase\s+([0-9a-fA-F]+)", out).group(1), 16)
    size = int(re.search(r"SizeOfImage\s+([0-9a-fA-F]+)", out).group(1), 16)
    return base, size


def load_modules(bases_path, dll_dir):
    modules = []
    for line in open(bases_path):
        address, name = line.split(maxsplit=1)
        name = name.strip()
        path = os.path.join(dll_dir, name)
        if os.path.exists(path):
            image_base, size = image_info(path)
            modules.append((int(address, 16), size, name, path, image_base))
    return modules


def parse(sample_path, thread):
    rows, current = [], None
    for line in open(sample_path, errors="replace"):
        match = PATTERN.match(line.rstrip("\n"))
        if not match:
            continue
        depth, count, text = len(match.group(1)), int(match.group(2)), match.group(3)
        if text.startswith("Thread_"):
            current = text.split()[0]
            continue
        if current == thread:
            rows.append((depth, count, text))
    return rows


def frame_key(text, modules, pending):
    address = ADDRESS.search(text)
    if address and int(address.group(1), 16) < 1 << 32:
        value = int(address.group(1), 16)
        for base, size, name, path, image_base in modules:
            if base <= value < base + size:
                pending.setdefault(path, set()).add(value - base + image_base)
                return ("x86", path, value - base + image_base, name)
        return ("text", "x86 code outside the known DLLs")
    named = NAMED.match(text)
    if named and not named.group(1).startswith("???"):
        return ("text", f"{named.group(1).split(' + ')[0]} ({named.group(2)})")
    if named:
        return ("text", f"??? ({named.group(2)})")
    return ("text", text[:60])


def symbolize(pending):
    names = {}
    for path, addresses in pending.items():
        ordered = sorted(addresses)
        query = "\n".join(hex(a) for a in ordered) + "\n"
        out = subprocess.run([os.environ.get("LLVM_SYMBOLIZER", "llvm-symbolizer"), "--obj", path,
                              "--functions=short", "--no-inlines"], input=query, capture_output=True, text=True).stdout
        blocks = [b for b in out.strip().split("\n\n")]
        for address, block in zip(ordered, blocks):
            lines = block.strip().split("\n")
            names[(path, address)] = lines[0] if lines else "?"
    return names


def main(sample_path, thread, bases_path, dll_dir, top=40):
    modules = load_modules(bases_path, dll_dir)
    rows = parse(sample_path, thread)
    pending, keys = {}, []
    for depth, count, text in rows:
        keys.append(frame_key(text, modules, pending))
    names = symbolize(pending)

    def label(key):
        if key[0] == "x86":
            return f"{names.get((key[1], key[2]), '?')} ({key[3]})"
        return key[1]

    self_time, inclusive, module_time = collections.Counter(), collections.Counter(), collections.Counter()
    stack = []
    total = 0
    for i, (depth, count, text) in enumerate(rows):
        while stack and stack[-1][0] >= depth:
            stack.pop()
        stack.append((depth, label(keys[i])))
        children, child_depth = 0, None
        for later_depth, later_count, _ in rows[i + 1:]:
            if later_depth <= depth:
                break
            child_depth = child_depth or later_depth
            if later_depth == child_depth:
                children += later_count
        if count > children:
            own = count - children
            total += own
            self_time[stack[-1][1]] += own
            for name in {name for _, name in stack}:
                inclusive[name] += own
            for module in {name.rsplit("(", 1)[-1].rstrip(")") for _, name in stack if name.endswith(")")}:
                module_time[module] += own
    print(f"{sample_path} {thread}: {total} samples")
    print("\nmodules, inclusive (any frame of the sample in the module):")
    for name, count in module_time.most_common(int(top)):
        print(f"  {100 * count / total:5.1f}%  {name}")
    print("\nself:")
    for name, count in self_time.most_common(int(top)):
        print(f"  {100 * count / total:5.1f}%  {name}")
    print("\ninclusive:")
    for name, count in inclusive.most_common(int(top)):
        print(f"  {100 * count / total:5.1f}%  {name}")


main(*sys.argv[1:])
