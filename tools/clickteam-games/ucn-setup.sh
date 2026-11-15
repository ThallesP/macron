#!/bin/bash
# ucn-setup.sh: prepares ~/ucn on a Mac sandbox for ucn-run.sh, for Ultimate
# Custom Night (Steam 871720) already downloaded by Steam and started once
# through macron (so its prefix exists). Nothing here starts Steam or touches
# Steam.app; the game's files stay where Steam put them.
#   ~/ucn/dll/{m5,stock}/: macron.5's wined3d.dll (i386, x86_64) and
#     wow64cpu.dll as installed, and Gcenx's stock ones from the cached
#     wine-devel-11.21 archive, with their SHA-256 in ~/ucn/dll/sha256.txt.
#   ~/ucn/pfx-apple: a copy of Steam's prefix for the game, which an earlier
#     MACRON_OPENGL=apple-software start left with the Mac driver's
#     AllowSoftwareRendering=y (macron writes it and never takes it out).
#   ~/ucn/pfx-default: the same without that value, so Wine's OpenGL gets no
#     pixel format on a Mac without a hardware renderer, as on any start
#     without the switch.
set -eu
U=$HOME/ucn
M="$HOME/Library/Application Support/Macron"
P="$HOME/Library/Application Support/Steam/steamapps/compatdata/871720/pfx"
R=$(ls -d "$M"/runtimes/*/)wine-dxmt/lib/wine
mkdir -p "$U"/dll/m5/i386 "$U"/dll/m5/x64 "$U"/dll/stock/i386 "$U"/dll/stock/x64 "$U"/runs "$U"/tmp
cp "$R/i386-windows/wined3d.dll" "$U/dll/m5/i386/"
cp "$R/x86_64-windows/wined3d.dll" "$R/x86_64-windows/wow64cpu.dll" "$U/dll/m5/x64/"
src="Wine Devel.app/Contents/Resources/wine/lib/wine"
tar -xJf "$(ls "$M"/downloads/9909b486*/wine-devel-11.21-osx64.tar.xz)" -C "$U/tmp" \
  "$src/i386-windows/wined3d.dll" "$src/x86_64-windows/wined3d.dll" "$src/x86_64-windows/wow64cpu.dll"
cp "$U/tmp/$src/i386-windows/wined3d.dll" "$U/dll/stock/i386/"
cp "$U/tmp/$src/x86_64-windows/wined3d.dll" "$U/tmp/$src/x86_64-windows/wow64cpu.dll" "$U/dll/stock/x64/"
rm -rf "$U/tmp"
(cd "$U/dll" && shasum -a 256 */*/*.dll >sha256.txt && cat sha256.txt)

pkill -9 -f 'wineserver|winedevice|Custom Night.exe|Macron/runtimes' 2>/dev/null || true
rm -rf "$U/pfx-apple" "$U/pfx-default"
cp -cR "$P" "$U/pfx-apple"
cp -cR "$P" "$U/pfx-default"
python3 -I - "$U/pfx-default/user.reg" <<'EOF'
import sys
path = sys.argv[1]
out, skip = [], False
for line in open(path, encoding="utf-8", errors="surrogateescape"):
    if line.startswith("["):
        skip = line.startswith("[Software\\\\Wine\\\\Mac Driver]")
    if not skip:
        out.append(line)
open(path, "w", encoding="utf-8", errors="surrogateescape").writelines(out)
EOF
printf null >"$U/pfx-default/.macron-registry.json"
grep -c AllowSoftwareRendering "$U/pfx-apple/user.reg" "$U/pfx-default/user.reg" || true
