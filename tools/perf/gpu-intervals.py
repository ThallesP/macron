"""Sums a Metal System Trace's GPU work from an `xctrace export` of the
metal-gpu-intervals table: total time on each GPU channel (vertex, fragment,
compute, blit), per command buffer, and per encoder label.

    python3 -I tools/perf/gpu-intervals.py <metal-gpu-intervals.xml[.gz]>

xctrace writes a value once with id="N" and refers back to it with ref="N";
this resolves those. Only rows at nesting depth 0 are counted, so nested
intervals are not counted twice."""

import collections
import gzip
import statistics
import sys
import xml.etree.ElementTree as ET

COLUMNS = None


def text_of(element, seen):
    if "ref" in element.attrib:
        return seen.get(element.attrib["ref"], "")
    value = element.attrib.get("fmt") or (element.text or "")
    if "id" in element.attrib:
        seen[element.attrib["id"]] = value
    raw = (element.text or "").strip()
    if "id" in element.attrib and raw:
        seen[element.attrib["id"] + ":raw"] = raw
    return value


def raw_of(element, seen):
    if "ref" in element.attrib:
        return seen.get(element.attrib["ref"] + ":raw", "")
    return (element.text or "").strip()


def main(path):
    opener = gzip.open if path.endswith(".gz") else open
    with opener(path, "rb") as f:
        root = ET.parse(f).getroot()
    schema = root.find(".//schema")
    names = [col.findtext("mnemonic") for col in schema.findall("col")]
    seen = {}
    channel_ns = collections.Counter()
    per_buffer = collections.defaultdict(int)
    per_label = collections.Counter()
    first = last = None
    for row in root.iter("row"):
        cells = list(row)
        values = {}
        for name, cell in zip(names, cells):
            if cell.tag == "sentinel":
                values[name] = ""
                continue
            values[name] = text_of(cell, seen)
            values[name + ":raw"] = raw_of(cell, seen)
            for sub in cell.iter():
                if sub is not cell:
                    text_of(sub, seen)
        if values.get("event-depth:raw", "0") not in ("0", ""):
            continue
        try:
            duration = int(values.get("duration:raw") or 0)
            start = int(values.get("start:raw") or 0)
        except ValueError:
            continue
        channel = values.get("channel-name", "?")
        channel_ns[channel] += duration
        per_buffer[values.get("cmdbuffer-id:raw") or values.get("cmdbuffer-id", "?")] += duration
        label = values.get("event-label", "").split("(")[0].strip()
        per_label[(channel, label)] += duration
        first = start if first is None else min(first, start)
        last = start + duration if last is None else max(last, start + duration)
    span_s = (last - first) / 1e9 if first is not None else 0
    print(f"trace span {span_s:.2f} s, command buffers {len(per_buffer)}")
    for channel, ns in channel_ns.most_common():
        print(f"  {channel}: {ns / 1e6:.1f} ms in all, {ns / 1e6 / span_s:.2f} ms per second of trace")
    buffers = [ns / 1e6 for ns in per_buffer.values()]
    if buffers:
        print(f"  per command buffer: median {statistics.median(buffers):.3f} ms, max {max(buffers):.3f} ms")
    print("  by channel and label (top 10):")
    for (channel, label), ns in per_label.most_common(10):
        print(f"    {channel:10} {label[:50]:50} {ns / 1e6:9.1f} ms")


if __name__ == "__main__":
    main(sys.argv[1])
