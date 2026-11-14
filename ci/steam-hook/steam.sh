#!/bin/bash
set -euo pipefail

steam_dir="$HOME/Library/Application Support/Steam"
client="$steam_dir/Steam.AppBundle/Steam/Contents/MacOS/steam_osx"
login_log="$steam_dir/logs/steamui_login.txt"
windows="${WINDOWS_BIN:?WINDOWS_BIN must point at the compiled windows.swift}"

case "$1" in
  wait-login)
    since=$2
    deadline=$(( $(date +%s) + $3 ))
    while (( $(date +%s) < deadline )); do
      if [ -f "$login_log" ] \
        && tail -n +"$(( since + 1 ))" "$login_log" | grep -q 'SetLoginState: WaitingForCredentials' \
        && "$windows" | grep -q $'^Steam Helper\tSign in to Steam$'; then
        echo "Steam is at its login window:"
        "$windows" | grep -i steam
        tail -n +"$(( since + 1 ))" "$login_log" | grep 'WaitingForCredentials' | tail -1
        exit 0
      fi
      sleep 5
    done
    echo "::error::Steam did not reach its login window in $3 s"
    "$windows" || true
    tail -20 "$login_log" 2>/dev/null || true
    tail -20 "$steam_dir/logs/bootstrap_log.txt" 2>/dev/null || true
    exit 1
    ;;
  shutdown)
    "$client" -shutdown > /dev/null 2>&1 &
    for _ in $(seq 1 60); do
      pgrep -x steam_osx > /dev/null || { echo "Steam has quit"; exit 0; }
      sleep 1
    done
    echo "::error::Steam did not quit within 60 s of -shutdown"
    exit 1
    ;;
  *)
    echo "usage: steam.sh wait-login <lines> <seconds> | shutdown" >&2
    exit 2
    ;;
esac
