# The Steam Play switch

Steam for Mac ships the same compatibility-tool code as Steam for Linux (Steam Play: `CCompatManager`, the `compatibilitytools.d` scan, the Install and Play paths for apps of another platform), but switched off. This folder is the small library that switches it on, so Steam's own Install and Play buttons work for Windows-only games with Macron as the tool. It is Macron's own code; NotProton (github.com/NotProtonNot/NotProton) showed that the switch exists and how a library gets into the client, and nothing of its source is in here.

## What it changes

Two instructions in `steamclient.dylib`, in memory, in the running client. Nothing on disk.

1. **Enabled.** The compat manager's constructor decides once whether Steam Play is on: it compares this platform's name with `"linux"` and stores the answer in its `enabled` flag, which every other compat function reads first. The instruction that turns the comparison into the answer (`cset w8, eq`) becomes `mov w8, #1`. With the flag on, Steam registers the tools it finds (it still ignores every Proton, "for a different target platform linux"), offers Install for Windows-only apps that a tool covers, downloads their Windows depots into its normal library, and starts them through the tool. Apps with a Mac version keep installing and starting natively: the same code decides that on Linux for native Linux games.
2. **Stop.** Steam's Stop button asks macOS to quit the game as an application (`-[NSRunningApplication terminate]`), then would go on to SIGINT, SIGTERM and a kill. When the process it started is not a Mac application, which `macron` is not, the first step fails and Steam gives up, leaving the game running and the page on "Stopping". The branch that gives up (`tbnz w0, #0, …` after the call) becomes an unconditional branch to the next step, so Steam carries on down its own ladder and `macron` gets the SIGINT. A Mac application still gets its quit request exactly as before.

Steam also has to find Macron's tool. The Mac client scans `/usr/share/steam/compatibilitytools.d`, `/usr/local/share/steam/compatibilitytools.d`, its own program folder and `STEAM_EXTRA_COMPAT_TOOLS_PATHS`, not `~/Library/Application Support/Steam/compatibilitytools.d` where Macron registers; `macron steam enable` sets that variable for Steam.

## Only builds it knows

`builds.h` is generated from `internal/steamhook/builds.go` (`go run ./cmd/steam-builds --header-only steamplay/builds.h`; a Go test fails when they differ). Each row is one `steamclient.dylib`, named by its `LC_UUID`, with the address, the instruction that must be there and the instruction that replaces it. When `steamclient.dylib` is loaded, the library reads its `LC_UUID`:

- not in the table: it writes `UNKNOWN_STEAM_BUILD` to `~/Library/Application Support/Macron/logs/steamplay.log` and changes nothing. Steam runs as stock Steam.
- in the table, but either instruction is not the expected one: `UNEXPECTED_STEAM_CODE`, and it changes nothing.
- otherwise it changes both and logs `Steam Play is on`.

`cmd/steam-builds` finds both instructions in any `steamclient.dylib` from Valve's update CDN, on Linux, and refuses a binary in which either is missing or ambiguous (`internal/steamhook/clientcode.go`). That is how a new Steam build gets its row: README.md, "Steam hook".

## How it gets into Steam

`macron steam enable` puts the library at `Steam.app/Contents/MacOS/macron-steamplay.dylib` and names it in `LSEnvironment` of `Steam.app/Contents/Info.plist` (`DYLD_INSERT_LIBRARIES`), the way macOS documents for loading a library into an application. Steam.app's `steam_osx` is signed with the hardened runtime, under which macOS ignores that variable, and `Info.plist` is covered by the signature, so the bundle is then signed again ad hoc (`codesign -f -s -`). That is the one change to Valve's files on disk: `Info.plist`, the signature inside `steam_osx`, and `_CodeSignature/CodeResources`. `macron steam enable` keeps the three originals and `macron steam disable` puts them back byte for byte, after which Valve's notarised signature verifies again.

Less invasive ways, and why each one is not enough:
- **Registration alone** (`compatibilitytools.d` plus the mapping in `config.vdf`). Tried on a logged-in Steam: it reads the mapping and still refuses Windows-only apps ("Invalid platform"), because the flag above is off.
- **A setting or a command-line flag.** From the client's code: the flag is computed from the platform name and nothing else. The one input that turns it on is telling the client it runs on Linux, which changes everything else too.
- **The variable without signing again** (`LSEnvironment` alone, or `launchctl setenv`). Apple's rule for the hardened runtime is that `DYLD_INSERT_LIBRARIES` is ignored unless the program carries an entitlement Steam.app does not have. The client inside `~/Library/Application Support/Steam` is not hardened and takes the library (tried on a sandbox, started directly), but a person starts Steam.app, whose hardened `steam_osx` starts that client.

Inside Steam, the library stays out of everything else. `steam_osx` replaces itself twice on the way up (Steam.app's bootstrapper, then the client), so the library takes itself out of `DYLD_INSERT_LIBRARIES` as soon as it is loaded and puts itself back only when the program being started is named `steam_osx` (dyld interposing of `execv`, `execve`, `execvp`, `posix_spawn` and `posix_spawnp`). Steam's web helper, macOS's own tools and every game start without it.

## What a sandbox cannot show

GitHub's Mac runners have System Integrity Protection off, and there macOS accepts an inserted library even in a hardened program (Valve's untouched Steam.app loaded a test library on a sandbox). So a sandbox cannot show that the ad hoc signature is needed, nor that it is enough, on a Mac with SIP on. Both rest on Apple's documented rule for the hardened runtime and on NotProton doing the same on its users' Macs. The same goes for macOS's App Management protection, which may ask before another program changes Steam.app: the sandbox never asks.

## Building

`steamplay/build.sh <out.dylib>` on a Mac (clang, both architectures; dyld stops a program whose inserted library lacks its architecture). `ci/package/build.sh` puts it in the installer at `/usr/local/lib/macron/`; `macron` looks for it in `MACRON_STEAMPLAY_LIBRARY`, beside itself, then in `../lib/macron/`.
