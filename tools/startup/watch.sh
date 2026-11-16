#!/bin/bash
# watch.sh <out file>: every 0.3 s, the windows on screen and the apps macOS
# counts as Dock apps, with a time stamp, while ~/s/watching exists.
out=$1
: > "$out"
while [ -e "$HOME/s/watching" ]; do
  t=$(perl -MTime::HiRes=time -e 'printf "%.3f", time')
  "$HOME/s/drive" windows 2>/dev/null | sed "s/^/$t window /" >> "$out"
  lsappinfo list 2>/dev/null | awk -v t="$t" '/^ *[0-9]+\) "/ {name=$0} /type="Foreground"/ {print t, "dock", name}' >> "$out"
  sleep 0.3
done
