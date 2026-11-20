#!/bin/bash
# Runs Wine's sound tests against two winecoreaudio.so builds in alternating
# rounds, with the Wine of an installed Macron runtime, and prints one line a
# run: the test's own summary and which checks failed.
#
#   tools/wine-tests/sound.sh <work-dir> <macron home> <before.so> <after.so> [rounds]
#
# <work-dir> is build.sh's; <macron home> holds the runtime (`macron runtime
# install` with MACRON_HOME set to it); <before.so> is usually Gcenx's
# winecoreaudio.so, <after.so> the patched one. Both Wine trees get the file
# under test, and <before.so> is left installed at the end.
#
# Several checks wait 20 to 40 ms for something a timer does (render.c:1282,
# 1288, 1297), and a sandbox's timers are late, so they fail now and then
# with any driver: that is why this runs rounds and compares, and why one
# run of one build says little. A check that fails in every round of one
# build and in none of the other is a finding.
set -u

work=$(cd "$1" && pwd)
home=$(cd "$2" && pwd)
before=$3
after=$4
rounds=${5:-3}
runtime=$(ls -d "$home"/runtimes/wine-*/ | head -1)
wine="$runtime/wine-dxmt/bin/wine"
tests=$work/build-tests/dlls
out=$work/wine-tests
mkdir -p "$out"
export WINEPREFIX=$out/prefix WINEDEBUG=-all MACRON_AUDIO_REPORT=0

# A copy beside the file, then a rename: a Wine process that still has the old
# file mapped keeps it.
put() {
  for tree in wine wine-dxmt; do
    cp "$1" "$runtime/$tree/lib/wine/x86_64-unix/winecoreaudio.so.new"
    mv "$runtime/$tree/lib/wine/x86_64-unix/winecoreaudio.so.new" "$runtime/$tree/lib/wine/x86_64-unix/winecoreaudio.so"
  done
}

one() { # <tag> <module> <test>
  local log="$out/$1-$2-$3.txt"
  (cd "$tests/$2/tests/x86_64-windows" && perl -e 'alarm shift; exec @ARGV' 300 "$wine" "$2_test.exe" "$3") >"$log" 2>&1
  local status=$?
  local summary failed
  summary=$(grep -a 'tests executed' "$log" | tail -1 | tr -d '\r' | cut -d: -f2-)
  failed=$(grep -a 'Test failed' "$log" | sed -E 's/^([a-z0-9]+\.c:[0-9]+).*/\1/' | sort | uniq -c | tr -s ' ' | tr '\n' ',')
  echo "$1 $2:$3 exit $status;${summary:- no summary}; failed:${failed:- none}"
}

for round in $(seq 1 "$rounds"); do
  for side in before after; do
    if [ "$side" = before ]; then put "$before"; else put "$after"; fi
    one "$side-$round" mmdevapi render
    one "$side-$round" winmm wave
    one "$side-$round" dsound dsound
    one "$side-$round" dsound dsound8
  done
done
put "$before"
"$runtime/wine-dxmt/bin/wineserver" -k 2>/dev/null
