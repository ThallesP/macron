#!/bin/bash
# Drives the real Steam window on a Mac where nobody works (an sbx sandbox or a
# CI runner): signs in to the test account, adds, installs, starts and stops a
# game with Steam's own buttons, and keeps a masked screenshot of each step.
#
#   steam.sh <step> [arguments]     (run it with no step for the list)
#
# DRIVE is the compiled ci/steam-play/drive.swift, EVIDENCE the folder the
# screenshots go to. STEAM_TEST_ACCOUNT is a mode-600 file with STEAM_TEST_USER
# and STEAM_TEST_PASSWORD; nothing here prints it. Buttons are found by the
# text macOS reads in the window, so no step depends on a window size.
set -uo pipefail

drive=${DRIVE:?DRIVE must point at the compiled drive.swift}
evidence=${EVIDENCE:?EVIDENCE must name a folder for the screenshots}
steam_app=${STEAM_APP:-/Applications/Steam.app}
steam_dir="$HOME/Library/Application Support/Steam"
client="$steam_dir/Steam.AppBundle/Steam/Contents/MacOS/steam_osx"
helper="Steam Helper"
mkdir -p "$evidence"

# The top bar and the activity feed show the account's name.
library_masks=(360,0,664,65 280,430,744,300)
store_masks=(360,0,664,65)

say() {
  echo "steam.sh: $*"
}

fail() {
  echo "::error::steam.sh: $*"
  shot_main "failed-$(date +%H%M%S)" "${library_masks[@]}"
  exit 1
}

has_window() {
  "$drive" windows | awk -F'\t' -v owner="$1" -v title="$2" '$4 == owner && $5 == title { found = 1 } END { exit !found }'
}

wait_window() {
  local owner=$1 title=$2 seconds=$3
  for _ in $(seq 1 $((seconds / 2))); do
    has_window "$owner" "$title" && return 0
    sleep 2
  done
  return 1
}

wait_no_window() {
  local owner=$1 title=$2 seconds=$3
  for _ in $(seq 1 $((seconds / 2))); do
    has_window "$owner" "$title" || return 0
    sleep 2
  done
  return 1
}

sees() {
  "$drive" find "$helper" Steam "$1" > /dev/null 2>&1
}

wait_text() {
  local text=$1 seconds=$2
  for _ in $(seq 1 $((seconds / 3))); do
    sees "$text" && return 0
    sleep 3
  done
  return 1
}

# Whether the game page's big button says `text`. The button is the leftmost
# thing on its row; "PLAY TIME" further right is a caption, not the button.
button_says() {
  local place origin
  place=$("$drive" find "$helper" Steam "$1" 2> /dev/null) || return 1
  origin=$("$drive" windows | awk -F'\t' -v owner="$helper" '$4 == owner && $5 == "Steam" { print $3 }')
  [ $((${place%% *} - ${origin%%,*})) -lt 460 ]
}

wait_button() {
  local text=$1 seconds=$2
  for _ in $(seq 1 $((seconds / 3))); do
    button_says "$text" && return 0
    sleep 3
  done
  return 1
}

click_text() {
  "$drive" click-text "$helper" Steam "$1" "${2:-first}" > /dev/null 2>&1
}

shot_main() {
  local name=$1
  shift
  "$drive" shot "$helper" Steam "$evidence/$name.png" "$@" > /dev/null 2>&1 || say "no Steam window to capture for $name"
}

# Steam comes to the front through its Dock icon: macOS ignores a background
# process that asks for another app to be activated.
front() {
  osascript -e 'tell application "System Events" to tell process "Dock" to click (first UI element of list 1 whose name is "Steam")' > /dev/null 2>&1
  sleep 2
}

log_lines() {
  wc -l < "$steam_dir/logs/$1" 2> /dev/null || echo 0
}

log_since() {
  tail -n +"$(($2 + 1))" "$steam_dir/logs/$1" 2> /dev/null
}

dismiss_system_dialogs() {
  for _ in 1 2 3 4 5 6; do
    local place
    place=$("$drive" windows | awk -F'\t' '$4 == "UserNotificationCenter" { print $3; exit }')
    [ -z "$place" ] && return 0
    local origin=${place%% *}
    "$drive" click $((${origin%,*} + 188)) $((${origin#*,} + 219))
    sleep 2
  done
}

step_start() {
  open -a "$steam_app"
  for _ in $(seq 1 450); do
    has_window "$helper" Steam && break
    has_window "$helper" "Sign in to Steam" && break
    sleep 2
  done
  sleep 6
  dismiss_system_dialogs
  has_window "$helper" Steam && { say "Steam is at its main window"; return 0; }
  has_window "$helper" "Sign in to Steam" && { say "Steam is at its login window"; return 0; }
  fail "Steam reached neither its login window nor its main window"
}

step_quit() {
  "$client" -shutdown > /dev/null 2>&1 &
  for _ in $(seq 1 90); do
    pgrep -x steam_osx > /dev/null || { say "Steam has quit"; return 0; }
    sleep 1
  done
  fail "Steam did not quit within 90 s of -shutdown"
}

# Types the test account into the login window. A Steam Guard prompt or a
# CAPTCHA stops the run: nobody guesses a code or retries in a loop.
step_login() {
  local account=${STEAM_TEST_ACCOUNT:?STEAM_TEST_ACCOUNT must name the account file}
  [ "$(stat -f %Lp "$account")" = 600 ] || fail "$account must be mode 600"
  has_window "$helper" Steam && { say "already signed in"; return 0; }
  has_window "$helper" "Sign in to Steam" || fail "no login window"
  dismiss_system_dialogs

  local connection_lines place origin x y
  connection_lines=$(log_lines connection_log.txt)
  place=$("$drive" windows | awk -F'\t' -v owner="$helper" '$4 == owner && $5 == "Sign in to Steam" { print $3 }')
  origin=${place%% *}
  x=${origin%,*}
  y=${origin#*,}
  "$drive" click $((x + 230)) $((y + 139))
  sleep 0.5
  "$drive" key cmd+a delete
  "$drive" type-value "$account" STEAM_TEST_USER
  "$drive" click $((x + 230)) $((y + 213))
  sleep 0.5
  "$drive" key cmd+a delete
  "$drive" type-value "$account" STEAM_TEST_PASSWORD
  "$drive" key return

  for _ in $(seq 1 60); do
    log_since connection_log.txt "$connection_lines" | grep -q 'Logged On' && break
    sleep 2
  done
  if ! log_since connection_log.txt "$connection_lines" | grep -q 'Logged On'; then
    grep -o 'SetLoginState: [A-Za-z]*' "$steam_dir/logs/steamui_login.txt" | tail -3
    fail "Steam did not sign in. If it asks for a Steam Guard code or shows a CAPTCHA, that is for Thalles (PROGRESS.md, Needs Thalles); do not retry"
  fi
  wait_window "$helper" Steam 120 || fail "signed in, but the main window did not open"
  sleep 8
  dismiss_system_dialogs
  say "signed in to the test account"
}

# Adds a free game to the account with the store page's own button.
step_add() {
  local app_id=$1 is_offered=0
  front
  open "steam://store/$app_id"
  sleep 8
  for _ in $(seq 1 12); do
    if sees "is already in your Steam library"; then
      say "app $app_id is already in the library"
      return 0
    fi
    sees "Add to Library" && { is_offered=1; break; }
    "$drive" scroll 400 400 -6
    sleep 3
  done
  [ "$is_offered" = 1 ] || fail "the store page of $app_id has no Add to Library button"
  click_text "Add to Library"
  wait_text "has been added to your account" 30 || fail "app $app_id was not added to the account"
  shot_main "added-$app_id" "${store_masks[@]}"
  click_text OK
  sleep 2
  say "app $app_id added to the account"
}

step_page() {
  local app_id=$1 name=$2
  front
  open "steam://nav/games/details/$app_id"
  wait_text "Store Page" 40 || fail "the library page of $app_id did not open"
  sleep 3
  click_text "No Thanks"
  shot_main "$name" "${library_masks[@]}"
}

# When another machine signed in to the account has the game, Steam's button
# offers to stream from it (STREAM, or CONNECT while it runs there) and Install
# is behind the arrow beside the button, as "This device". The arrow sits just
# left of the SPACE REQUIRED caption.
choose_this_device() {
  local place
  button_says INSTALL && return 0
  place=$("$drive" find "$helper" Steam "SPACE REQUIRED" 2> /dev/null) || return 1
  "$drive" click $((${place%% *} - 92)) $((${place##* } + 8))
  sleep 2
  click_text "This device" || return 1
  say "another machine of the account has this game; chose This device"
  wait_button INSTALL 20
}

# Presses Install on the game's library page and Install in Steam's dialog,
# then waits for Steam to report the app fully installed.
step_install() {
  local app_id=$1 seconds=${2:-1800} content_lines
  step_page "$app_id" "install-$app_id-1-page"
  sees "SPACE REQUIRED" || fail "the Install button of $app_id is not active (the page does not show the space it needs)"
  choose_this_device || fail "the button of $app_id does not offer Install on this machine"
  content_lines=$(log_lines content_log.txt)
  click_text INSTALL || fail "no INSTALL button on the page of $app_id"
  wait_text "INSTALL TO:" 30 || fail "Steam's install dialog did not open for $app_id"
  shot_main "install-$app_id-2-dialog" "${library_masks[@]}"
  click_text Install last || fail "no Install button in the dialog"

  for _ in $(seq 1 $((seconds / 5))); do
    log_since content_log.txt "$content_lines" | grep -q "AppID $app_id state changed : Fully Installed," && break
    log_since content_log.txt "$content_lines" | grep "AppID $app_id" | grep -qi "update canceled\|failed" && break
    sleep 5
  done
  log_since content_log.txt "$content_lines" | grep "AppID $app_id" | tail -3
  log_since content_log.txt "$content_lines" | grep -q "AppID $app_id state changed : Fully Installed," || fail "Steam did not finish installing $app_id"
  [ -f "$steam_dir/steamapps/appmanifest_$app_id.acf" ] || fail "no appmanifest_$app_id.acf in Steam's library"
  wait_button PLAY 60 || fail "the page of $app_id shows no PLAY button after the install"
  shot_main "install-$app_id-3-installed" "${library_masks[@]}"
  say "Steam installed $app_id into its library; the page shows PLAY"
}

# A Steam without Steam Play offers neither Play nor a local install for a
# Windows-only game: its page says AVAILABLE FOR Windows, or, when another
# machine of the account has the game, offers to stream from there.
step_stock() {
  local app_id=$1 name=$2
  step_page "$app_id" "$name"
  button_says PLAY && fail "Steam offers to play $app_id on this machine"
  sees "SPACE REQUIRED" && fail "Steam offers to install $app_id on this machine"
  if sees "AVAILABLE FOR"; then
    say "stock Steam: $app_id is AVAILABLE FOR another platform, Install is greyed out"
    return 0
  fi
  if button_says STREAM || button_says CONNECT; then
    say "stock Steam: $app_id cannot be installed here; Steam only offers to stream it from another machine of the account"
    return 0
  fi
  fail "the page of $app_id shows neither AVAILABLE FOR nor a stream from another machine"
}

# Presses Play and waits for the game's window. Steam then says Running and
# offers STOP.
step_play() {
  local app_id=$1 owner=$2 title=$3 seconds=${4:-300} process_lines
  step_page "$app_id" "play-$app_id-1-page"
  process_lines=$(log_lines gameprocess_log.txt)
  button_says PLAY || fail "no PLAY button on the page of $app_id"
  click_text PLAY
  wait_window "$owner" "$title" "$seconds" || fail "the window $owner / $title did not open within $seconds s of Play"
  sleep 6
  "$drive" shot "$owner" "$title" "$evidence/play-$app_id-2-game.png" > /dev/null 2>&1
  front
  wait_button STOP 30 || fail "Steam does not show $app_id as running"
  shot_main "play-$app_id-3-running" "${library_masks[@]}"
  log_since gameprocess_log.txt "$process_lines" | grep -q "Remove $app_id from running list" && fail "Steam took $app_id off its running list while the game is up"
  say "$app_id is running: its window is open and Steam shows STOP"
}

# Quits the game the way its Dock menu's Quit does, and waits for Steam to go
# back to PLAY.
step_quit_game() {
  local app_id=$1 owner=$2 title=$3
  "$drive" quit "$owner" || fail "no running application named $owner to quit"
  wait_no_window "$owner" "$title" 60 || fail "the game's window is still open 60 s after Quit"
  front
  wait_button PLAY 90 || fail "Steam did not return to PLAY after the game quit"
  shot_main "quit-$app_id-back-to-play" "${library_masks[@]}"
  say "$app_id quit; Steam is back at PLAY"
}

# Presses Steam's STOP and confirms.
step_stop() {
  local app_id=$1 owner=$2 title=$3
  front
  click_text STOP || fail "no STOP button on the page of $app_id"
  wait_text "Exit game?" 20 || fail "Steam did not ask to confirm the stop"
  click_text Confirm || fail "no Confirm button"
  wait_no_window "$owner" "$title" 60 || fail "the game's window is still open 60 s after Stop"
  wait_button PLAY 90 || fail "Steam did not return to PLAY after Stop"
  shot_main "stop-$app_id-back-to-play" "${library_masks[@]}"
  say "$app_id stopped from Steam; Steam is back at PLAY"
}

step=${1:-}
[ $# -gt 0 ] && shift
case "$step" in
  start | quit | login | add | page | stock | install | play | quit_game | stop) "step_$step" "$@" ;;
  *)
    echo "usage: steam.sh start | quit | login | add <app> | page <app> <name> | stock <app> <name> | install <app> [seconds] | play <app> <owner> <title> [seconds] | quit_game <app> <owner> <title> | stop <app> <owner> <title>" >&2
    exit 2
    ;;
esac
