#!/bin/bash
# m.sh <macron binary> <app id> <label>: one start of startup.exe through
# macron's Steam verb, as Steam would run it; prints when macron started,
# when the program's main() ran, and when macron returned.
s=$HOME/s
now() { perl -MTime::HiRes=time -e 'printf "%.3f", time'; }
m=$1 app=$2 label=$3
mkdir -p "$s/compat/$app" "$s/out"
rm -f "$s/startup-main.txt"
t0=$(now)
SteamAppId=$app STEAM_COMPAT_DATA_PATH=$s/compat/$app "$s/$m" waitforexitandrun "$s/startup.exe" 2 > "$s/out/$label.log" 2>&1
code=$?
t1=$(now)
main=$(awk '/STARTUP main/ {print $3}' "$s/startup-main.txt" 2>/dev/null)
perl -e 'printf "%-28s exit %d  to main() %6.2f s  to macron exit %6.2f s\n", @ARGV' "$label" $code "$(perl -e "print ${main:-0} - $t0")" "$(perl -e "print $t1 - $t0")"
