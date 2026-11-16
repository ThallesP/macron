#!/bin/bash
# ow-heap.sh <out> <pid> <tag>: what is in a running process's malloc heap.
#   heap-<tag>.txt      heap -s -H -z: objects by class and malloc zone, by size
#   vm-<tag>.txt        vmmap -summary
#   tree-<tag>.txt      malloc_history -callTree, top-down by caller (needs
#                       MallocStackLogging in the process's environment)
#   inv-<tag>.txt       the same tree inverted: by the frame that allocated
# The call trees take minutes on a large heap.
set -u
out=$1 pid=$2 tag=$3
mkdir -p "$out"
date -u +%FT%TZ > "$out/heap-$tag.time"
footprint "$pid" > "$out/fp-$tag.txt" 2>&1
vmmap -summary "$pid" > "$out/vm-$tag.txt" 2>&1
heap -s -H -z "$pid" > "$out/heap-$tag.txt" 2>&1
malloc_history "$pid" -callTree -consolidateSystemFramesBySymbol > "$out/tree-$tag.txt" 2>&1
malloc_history "$pid" -callTree -invert -consolidateSystemFramesBySymbol > "$out/inv-$tag.txt" 2>&1
date -u +%FT%TZ >> "$out/heap-$tag.time"
