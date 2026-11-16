#!/bin/sh
# Makes every Wine process of one Macron runtime tree an app that macOS
# counts as a game, so Game Mode can turn on for it; `revert` undoes it.
#
#   tools/perf/gamemode-bundle.sh apply|revert|status <wine tree>
#
# <wine tree> is a runtime's Wine folder, for example
# "$HOME/Library/Application Support/Macron/runtimes/<version>/wine-dxmt".
#
# How: Wine starts every Windows process, the game included, by running
# lib/wine/x86_64-unix/wine (ntdll's `wineloader`, the realpath of ntdll.so's
# folder plus "wine"). macOS's gamepolicyd labels a process a game from its
# app bundle's LSApplicationCategoryType; a bare executable is "anon<wine>"
# and is never labelled, even with the category in its embedded Info.plist
# (tried on a sandbox: still anon<wine>). So the loader moves into
# "Macron Game.app/Contents/MacOS/wine", with ntdll.so linked beside it (the
# loader looks for ntdll.so next to the realpath of itself), and the old
# path becomes a symlink to it. ntdll still resolves its own folder through
# the link, so the Windows DLL paths do not change.
#
# What the sandbox showed (docs/perf/README.md): gamepolicyd logs
# "Found game GameProcess(Optional("wine"), ..., labelReason=infoPlist)" for
# every Wine process, and Game Mode goes to "paused" while a windowed game has
# focus. The first start asks once whether "Macron Game" may find devices on
# local networks; Wine waits until the question is answered.
set -eu

[ $# -eq 2 ] || {
	echo "usage: gamemode-bundle.sh apply|revert|status <wine tree>" >&2
	exit 2
}
dir="$2/lib/wine/x86_64-unix"
app="$dir/Macron Game.app"
[ -e "$dir/ntdll.so" ] || {
	echo "gamemode-bundle: $dir has no ntdll.so; is $2 a Wine tree?" >&2
	exit 1
}

case $1 in
status)
	if [ -L "$dir/wine" ]; then echo "applied: $(readlink "$dir/wine")"; else echo "not applied"; fi
	;;
apply)
	if [ -L "$dir/wine" ]; then
		echo "already applied"
		exit 0
	fi
	mkdir -p "$app/Contents/MacOS"
	cp -p "$dir/wine" "$app/Contents/MacOS/wine"
	ln -sf "../../../ntdll.so" "$app/Contents/MacOS/ntdll.so"
	cat >"$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key><string>English</string>
  <key>CFBundleExecutable</key><string>wine</string>
  <key>CFBundleIdentifier</key><string>org.winehq.wine</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleName</key><string>Wine</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>11.21</string>
  <key>CFBundleVersion</key><string>11.21</string>
  <key>NSPrincipalClass</key><string>WineApplication</string>
  <key>LSUIElement</key><string>1</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.games</string>
  <key>LSSupportsGameMode</key><true/>
</dict>
</plist>
PLIST
	codesign -f -s - "$app" >/dev/null 2>&1
	mv "$dir/wine" "$dir/wine.before-gamemode"
	ln -s "Macron Game.app/Contents/MacOS/wine" "$dir/wine"
	echo "applied: Wine processes of $2 now run as $app"
	;;
revert)
	if [ ! -L "$dir/wine" ]; then
		echo "not applied"
		exit 0
	fi
	rm "$dir/wine"
	if [ -f "$dir/wine.before-gamemode" ]; then
		mv "$dir/wine.before-gamemode" "$dir/wine"
	else
		mv "$app/Contents/MacOS/wine" "$dir/wine"
	fi
	rm -rf "$app"
	echo "reverted"
	;;
*)
	echo "usage: gamemode-bundle.sh apply|revert|status <wine tree>" >&2
	exit 2
	;;
esac
