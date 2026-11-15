# Installing Macron

Macron comes as `Macron-<version>.dmg`. It holds `Macron.pkg`, which installs the `macron` program to `/usr/local/bin`, downloads Macron's Windows runtime into your home folder, and registers Macron with Steam.

## Before you start

1. Install Steam from <https://store.steampowered.com/about/> into `/Applications`, start it once so it finishes updating itself, and sign in.
2. Quit Steam (Steam > Quit Steam). Macron can only register itself while Steam is closed, because Steam rewrites its settings when it quits.

## The package is not notarised

Macron has no Apple Developer ID yet, so the package is unsigned and cannot be notarised: Apple only notarises software signed with a Developer ID. macOS 26 therefore refuses to open it with a double-click and says Apple could not verify it is free of malware. Pick one of these ways past that:

- **System Settings.** Double-click `Macron.pkg` and press Done on the warning. Open System Settings > Privacy & Security, scroll to Security, where it says "Macron.pkg" was blocked, and press Open Anyway, then confirm with your password or Touch ID. The installer opens.
- **Terminal, removing the quarantine.** Before opening the DMG, run `xattr -dr com.apple.quarantine ~/Downloads/Macron-<version>.dmg`, then open the DMG and double-click `Macron.pkg`.
- **Terminal, installing directly.** Open the DMG and run `sudo installer -pkg "/Volumes/Macron <version>/Macron.pkg" -target /`. The `installer` command does not ask Gatekeeper.

Control-click > Open, which used to skip the check, no longer does since macOS 15: it shows the same warning.

## What the install does

1. Copies `macron` (one universal binary, Apple silicon and Intel) and `macron-uninstall` to `/usr/local/bin`.
2. As you, not as root, runs `macron runtime install`: downloads Wine, DXMT and DXVK at pinned versions into `~/Library/Application Support/Macron` (about 1 GB, a few minutes).
3. If Steam is installed and closed, runs `macron steam enable`: adds Macron to `~/Library/Application Support/Steam/compatibilitytools.d` and makes it the compatibility tool for every app in Steam's `config.vdf`, after backing that file up. Nothing inside Steam's app is changed.

If Steam was running, step 3 is skipped and the log says so: quit Steam and run `macron steam enable` in Terminal. If Steam is a build Macron does not know yet (Steam updates itself), or any step fails, the install undoes itself and the log says why.

Everything the install does is written to `/Library/Logs/Macron-install.log` (open it with Console or `cat`).

## Uninstalling

Quit Steam, then run in Terminal:

```sh
sudo macron-uninstall
```

It runs `macron uninstall` as you, which puts Steam's `config.vdf` mapping back, removes Macron's folder from Steam's `compatibilitytools.d`, and deletes Macron's runtimes, Windows environments and logs from `~/Library/Application Support/Macron`, listing each path first. It never deletes anything Macron did not create: other files in that folder are left and named. Then it deletes `/usr/local/bin/macron`, `/usr/local/bin/macron-uninstall` and the install log.

Games started from Steam keep their Windows environment in Steam's own library (`steamapps/compatdata/<app id>/pfx`), next to their saves; the uninstall leaves those to Steam.
