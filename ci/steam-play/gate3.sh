#!/bin/bash
# GOAL.md gate 3 on a fresh Mac where nobody works, the way a person uses the
# release: Steam from Valve's DMG in /Applications, started and signed in
# first; then Macron's installer from the DMG; then gate 2's flow through
# Steam's own buttons; then sudo macron-uninstall, after which Steam must be
# as it was.
#
#   DRIVE=<drive> EVIDENCE=<dir> STEAM_TEST_ACCOUNT=<file> gate3.sh <Macron-x.y.z.dmg>
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
dmg=${1:?usage: gate3.sh <Macron-x.y.z.dmg>}
drive=${DRIVE:?}
evidence=${EVIDENCE:?}
steam_app=/Applications/Steam.app
steam_dir="$HOME/Library/Application Support/Steam"
macron_home="$HOME/Library/Application Support/Macron"
mkdir -p "$evidence"

say() {
  echo
  echo "== $*"
}

fail() {
  echo "::error::gate3: $*"
  exit 1
}

say "The installer under test"
shasum -a 256 "$dmg" | tee "$evidence/dmg.sha256"
[ ! -e /usr/local/bin/macron ] && [ ! -e "$macron_home" ] || fail "this Mac already has Macron; gate 3 starts without it"

GATE2_PHASE=before "$here/gate2.sh" || exit 1
cp "$steam_dir/config/config.vdf" "$evidence/config-before.vdf.private"

say "Install Macron.pkg from the DMG, with Steam open"
pgrep -x steam_osx > /dev/null || fail "Steam should be open for this check: the installer closes and reopens it"
mount=$(mktemp -d)
hdiutil attach -nobrowse -readonly -mountpoint "$mount" "$dmg" > /dev/null || fail "could not open $dmg"
ls -l "$mount"
sudo installer -pkg "$mount/Macron.pkg" -target / || fail "the installer failed"
hdiutil detach "$mount" > /dev/null
cp /Library/Logs/Macron-install.log "$evidence/Macron-install.log"
tail -5 /Library/Logs/Macron-install.log
grep -q '== Done: Steam Play is on with Macron as its tool' /Library/Logs/Macron-install.log || fail "the installer did not finish with Steam Play on"
/usr/local/bin/macron steam status | tee "$evidence/status-after-install.txt"
grep -q '^steam play:      on$' "$evidence/status-after-install.txt" || fail "Steam Play is not on after the install"
"$here/steam.sh" start || exit 1

MACRON=/usr/local/bin/macron GATE2_PHASE=after "$here/gate2.sh" || exit 1

say "sudo macron-uninstall, with Steam open"
sudo macron-uninstall | tee "$evidence/uninstall.txt" || fail "macron-uninstall failed"
grep -q "Valve's files are back, byte for byte" "$evidence/uninstall.txt" || fail "the uninstall did not restore Valve's files"

say "Steam is as it was"
(cd "$steam_app" && find . -type f -print0 | xargs -0 shasum -a 256 | sort -k 2) > "$evidence/steam-app-after.sha256"
diff "$evidence/steam-app-before.sha256" "$evidence/steam-app-after.sha256" || fail "Steam.app differs from before Macron"
codesign --verify --strict --deep "$steam_app" || fail "Steam.app's signature is not valid after the uninstall"
spctl -a -vv "$steam_app" 2>&1 | tee "$evidence/gatekeeper-after.txt"
grep -q 'Notarized Developer ID' "$evidence/gatekeeper-after.txt" || fail "Steam.app is not Valve's notarised app after the uninstall"
for left in /usr/local/bin/macron /usr/local/bin/macron-uninstall /usr/local/lib/macron "$macron_home" "$steam_dir/compatibilitytools.d/macron" "$steam_dir/config/config.vdf.before-macron"; do
  [ ! -e "$left" ] || fail "$left is still there after the uninstall"
done
grep -q '"macron"' "$steam_dir/config/config.vdf" && fail "config.vdf still names macron"
grep -c 'CompatToolMapping' "$evidence/config-before.vdf.private" "$steam_dir/config/config.vdf" | sed 's/.*config/config/'
rm -f "$evidence/config-before.vdf.private"
"$here/steam.sh" start || exit 1
"$here/steam.sh" page 871720 "9-after-uninstall-windows-game" || exit 1
"$drive" find "Steam Helper" Steam "AVAILABLE FOR" > /dev/null 2>&1 || fail "after the uninstall Steam should grey out the Windows-only game again"
"$drive" find "Steam Helper" Steam "SPACE REQUIRED" > /dev/null 2>&1 && fail "Steam still offers to install the Windows-only game after the uninstall"
say "Gate 3 flow passed"
