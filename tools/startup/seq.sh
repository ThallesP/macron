#!/bin/bash
# The start-up measurements on one sandbox, in order (docs/startup/README.md).
cd ~/s
mkdir -p out
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
compat=$HOME/s/compat
runtime_dir() { ./macron-new runtime path 2>/dev/null; }
older() { # <app>: make the prefix's record name another runtime, as after a Macron update
  perl -pi -e 's/"runtime":"[^"]*"/"runtime":"an-older-runtime"/' "$compat/$1/pfx/.macron-prefix.json"
  cat "$compat/$1/pfx/.macron-prefix.json"; echo
}
touch watching; ./watch.sh out/watch.txt & 
echo "== runtime install (rc3's macron)"; t=$(now); ./macron-rc3 runtime install > out/runtime-install.log 2>&1; echo "exit $? in $(perl -e "print $(now) - $t") s"
echo "== rc3"
./m.sh macron-rc3 1001 rc3-new-prefix
for i in 1 2 3; do ./m.sh macron-rc3 1001 rc3-ordinary-$i; done
older 1001; ./m.sh macron-rc3 1001 rc3-after-update
echo "== this branch"
for i in 1 2 3; do ./m.sh macron-new 1001 new-ordinary-$i; done
older 1001; ./m.sh macron-new 1001 new-after-update
./m.sh macron-new 1001 new-ordinary-after-update
echo "== templates (macron runtime install)"; t=$(now); ./macron-new runtime install > out/templates.log 2>&1; echo "exit $? in $(perl -e "print $(now) - $t") s"
./m.sh macron-new 1002 new-new-game-1
./m.sh macron-new 1003 new-new-game-2
mkdir -p "$compat/1004/pfx" && echo keep > "$compat/1004/pfx/keep.txt"
./m.sh macron-new 1004 new-boot-in-place
echo "== the server's linger"
rt=$(runtime_dir)/wine-dxmt
export WINEPREFIX=$compat/1001/pfx WINEDEBUG=-all
"$rt/bin/wine" cmd /c exit; t=$(now); "$rt/bin/wineserver" -w; echo "wineserver -w after a program, server started by Wine: $(perl -e "print $(now) - $t") s"
"$rt/bin/wineserver" -p0; "$rt/bin/wine" cmd /c exit; t=$(now); "$rt/bin/wineserver" -w; echo "wineserver -w after a program, server started with -p0: $(perl -e "print $(now) - $t") s"
unset WINEPREFIX
rm watching; wait
echo "SEQ-DONE"
