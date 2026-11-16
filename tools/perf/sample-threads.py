"""Splits each thread's samples in a macOS `sample` report into: inside a Wine
server call (the stack holds ntdll.so's server_call / wine_server_call),
blocked in a wait outside one (the deepest frame is a kernel wait), and
working (everything else, Rosetta-translated game code included).

    python3 -I tools/perf/sample-threads.py <sample.txt[.gz]> [threads]

Prints the threads with the most samples not blocked, and the ones with the
most samples inside server calls."""

import gzip
import re
import sys

WAITS = re.compile(
    r"(semaphore_wait_trap|semaphore_timedwait_trap|__psynch_cvwait|__psynch_mutexwait|mach_msg2_trap|mach_msg_trap|"
    r"__ulock_wait|__ulock_wait2|kevent|__select|__pselect|poll|__semwait_signal|__workq_kernreturn|"
    r"__recvmsg|__recvfrom|read|__read_nocancel|__sigsuspend|nanosleep|__wait4|__accept)\b"
)
SERVER = re.compile(r"\b(server_call_unlocked|wine_server_call|server_select|server_wait|wait_select_reply)\b")
LINE = re.compile(r"^([ +!:|]*)(\d+) (.*)$")


def main(path, show):
    opener = gzip.open if path.endswith(".gz") else open
    with opener(path, "rt", errors="replace") as f:
        lines = f.read().split("\n")
    start = next(i for i, l in enumerate(lines) if l.startswith("Call graph:")) + 1
    nodes = []  # (depth, count, text)
    for line in lines[start:]:
        if line.startswith("Total number in stack") or line.startswith("Sort by top of stack"):
            break
        m = LINE.match(line)
        if m:
            nodes.append((len(m.group(1)), int(m.group(2)), m.group(3)))
    threads = {}
    stack = []  # (depth, count, text, children_sum)
    current = None

    def close(entry):
        depth, count, text, children = entry
        self_count = count - children
        if self_count <= 0 or current is None:
            return
        path = [e[2] for e in stack] + [text]
        stats = threads[current]
        in_server = any(SERVER.search(p) for p in path)
        if in_server:
            stats["server"] += self_count
        elif WAITS.search(text.split("  (in ")[0]):
            stats["blocked"] += self_count
        else:
            stats["working"] += self_count

    for depth, count, text in nodes:
        while stack and stack[-1][0] >= depth:
            close(stack.pop())
        if text.startswith("Thread_"):
            current = text
            threads[current] = {"total": count, "server": 0, "blocked": 0, "working": 0}
            stack = [(depth, count, text, 0)]
            continue
        if stack:
            d, c, t, ch = stack[-1]
            stack[-1] = (d, c, t, ch + count)
        stack.append((depth, count, text, 0))
    while stack:
        close(stack.pop())

    def label(name):
        return name[:60]

    print("threads by samples working (not blocked, not in a server call):")
    for name, s in sorted(threads.items(), key=lambda kv: -kv[1]["working"])[:show]:
        print(f"  {s['working']:5} working {s['server']:5} in server calls {s['blocked']:5} blocked  {label(name)}")
    print("threads by samples inside server calls:")
    for name, s in sorted(threads.items(), key=lambda kv: -kv[1]["server"])[:show]:
        print(f"  {s['working']:5} working {s['server']:5} in server calls {s['blocked']:5} blocked  {label(name)}")


if __name__ == "__main__":
    main(sys.argv[1], int(sys.argv[2]) if len(sys.argv) > 2 else 8)
