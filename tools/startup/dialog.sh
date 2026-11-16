#!/bin/bash
# dialog.sh: what is on screen while wineboot updates a prefix, with Gcenx's
# wineboot (the wait window, the control) and with patch 0013's.
cd ~/s
mkdir -p out/dialog
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
rt=$(./macron-new runtime path)
mkdir -p overlay13 && tar -xJf wine-11.21-macron.13-overlay-osx64.tar.xz -C overlay13
shot_during() { # <label> <command...>: run it, screenshots and the window list every second while it runs
  local label=$1; shift
  touch watching; ./watch.sh "out/dialog/$label-watch.txt" &
  local t0=$(now)
  "$@" > "out/dialog/$label.log" 2>&1 &
  local pid=$! n=0
  while kill -0 $pid 2>/dev/null; do
    n=$((n+1)); screencapture -x "out/dialog/$label-$n.png" 2>/dev/null; sleep 1
  done
  wait $pid; local code=$?
  rm watching; wait
  echo "$label: exit $code in $(perl -e "print $(now) - $t0") s, $n screenshots"
  echo "  windows not owned by macOS's own apps:"; grep " window " "out/dialog/$label-watch.txt" | grep -v -E "	(Window Server|Dock|Control Center|SystemUIServer|Spotlight|Finder|Notification Center|TextInputMenuAgent|loginwindow|Wallpaper)	" | cut -d' ' -f3- | sort | uniq -c
  echo "  Dock apps seen:"; grep " dock " "out/dialog/$label-watch.txt" | cut -d' ' -f3- | sort | uniq -c
}
export WINEDEBUG=-all
p=$HOME/s/dialogpfx
for wineboot in gcenx macron13; do
  if [ $wineboot = macron13 ]; then
    for tree in wine wine-dxmt; do cp "$rt/$tree/lib/wine/x86_64-windows/wineboot.exe" "$rt/$tree/lib/wine/x86_64-windows/wineboot.exe.gcenx"; cp overlay13/lib/wine/x86_64-windows/wineboot.exe "$rt/$tree/lib/wine/x86_64-windows/wineboot.exe"; done
  fi
  rm -rf "$p"
  WINEPREFIX=$p shot_during $wineboot-new-prefix sh -c "\"$rt/wine-dxmt/bin/wine\" wineboot -u && \"$rt/wine-dxmt/bin/wineserver\" -w"
  echo 0 > "$p/.update-timestamp"
  WINEPREFIX=$p shot_during $wineboot-automatic-update sh -c "\"$rt/wine-dxmt/bin/wine\" cmd /c exit && \"$rt/wine-dxmt/bin/wineserver\" -w"
  echo "  .update-timestamp now $(cat "$p/.update-timestamp")"
  WINEPREFIX=$p shot_during $wineboot-forced-update sh -c "\"$rt/wine-dxmt/bin/wine\" wineboot -u && \"$rt/wine-dxmt/bin/wineserver\" -w"
done
echo "== control: patch 0013's wineboot with MACRON_WINEBOOT_WAIT_WINDOW=1"
WINEPREFIX=$p MACRON_WINEBOOT_WAIT_WINDOW=1 shot_during macron13-control-forced-update sh -c "\"$rt/wine-dxmt/bin/wine\" wineboot -u && \"$rt/wine-dxmt/bin/wineserver\" -w"
for tree in wine wine-dxmt; do mv "$rt/$tree/lib/wine/x86_64-windows/wineboot.exe.gcenx" "$rt/$tree/lib/wine/x86_64-windows/wineboot.exe"; done
echo DIALOG-DONE
