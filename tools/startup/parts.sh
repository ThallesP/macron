#!/bin/bash
# parts.sh: where an ordinary start's time goes inside Wine, prefix current.
cd ~/s
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
rt=$(./macron-new runtime path)/wine-dxmt
export WINEPREFIX=$HOME/s/compat/1001/pfx WINEDEBUG=-all WINEDLLOVERRIDES="mscoree=;mshtml=;winedbg.exe="
main_after() { perl -e 'printf "%.2f", $ARGV[0] - $ARGV[1]' "$(awk '/STARTUP main/ {print $3}' startup-main.txt)" "$1"; }
for i in 1 2 3; do
  "$rt/bin/wineserver" -w
  rm -f startup-main.txt; t=$(now); "$rt/bin/wine" "$HOME/s/startup.exe" 1 > /dev/null 2>&1; e=$(now)
  echo "cold, the program alone:               to main() $(main_after $t) s, to its exit $(perl -e "printf '%.2f', $e - $t") s"
  "$rt/bin/wineserver" -w
  rm -f startup-main.txt; t=$(now); "$rt/bin/wine" 'C:\windows\system32\steam.exe' "$HOME/s/startup.exe" 1 > /dev/null 2>&1; e=$(now)
  echo "cold, through steam.exe:               to main() $(main_after $t) s, to its exit $(perl -e "printf '%.2f', $e - $t") s"
  t=$(now); "$rt/bin/wineserver" -w; echo "  then wineserver -w:                   $(perl -e "printf '%.2f', $(now) - $t") s"
  "$rt/bin/wine" cmd /c exit > /dev/null 2>&1
  rm -f startup-main.txt; t=$(now); "$rt/bin/wine" "$HOME/s/startup.exe" 1 > /dev/null 2>&1; e=$(now)
  echo "warm (server and services running):    to main() $(main_after $t) s, to its exit $(perl -e "printf '%.2f', $e - $t") s"
  t=$(now); "$rt/bin/wine" cmd /c exit > /dev/null 2>&1; echo "  cold-ish wine cmd /c exit right after: $(perl -e "printf '%.2f', $(now) - $t") s"
done
"$rt/bin/wineserver" -w
echo "== a cold start's processes, WINEDEBUG=+process,+timestamp (first lines)"
rm -f startup-main.txt
WINEDEBUG=-all,+timestamp,+process "$rt/bin/wine" 'C:\windows\system32\steam.exe' "$HOME/s/startup.exe" 1 2>&1 | grep -E "create_process|CreateProcess|exec" | head -40 > out/process-trace.txt
"$rt/bin/wineserver" -w
echo PARTS-DONE
