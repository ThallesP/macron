# Hand-off: Steam's own Install and Play buttons

Written 2026-11-15 for the Claude Mythos 5.2 thread that takes over the Steam side. GOAL.md has the gates and the rules; PROGRESS.md has the evidence. This file says why the work is yours and what is already known.

## Why you

Thalles owns this project and wants the Proton experience on his own Mac: open the native Steam client, press Install, press Play. An Opus loop built everything around that (launcher, runtime with six Wine patches, scenario checks, an installer DMG that is built and tested but not published) and then parked the one piece the experience depends on, because its safety filter stopped it twice when it worked on the Steam client. Thalles's answer on 2026-11-15: "yeah see how NotProton does it, and if necessary delegate it to Mythos 5.2 to handle it". So this part is yours, and you are the only agent working in this repository right now; the Opus thread is idle.

## What is known

- Registration alone is not enough. With Macron in `compatibilitytools.d` and mapped as the default tool, the client logs `Mapping AppID 0 to tool "macron"` but a Windows-only game's Install button stays greyed out and a forced install fails with "Invalid platform" (`Steam_AppUpdateError_29`). The whole Steam Play system ships in `steamclient.dylib` and `steamui.dylib` and is switched off on macOS. See "Gate 2 findings" in PROGRESS.md.
- NotProton (github.com/NotProtonNot/NotProton, GPL-3.0, actively maintained, release 1.2.6) switches it on. From its source layout: it sets `DYLD_INSERT_LIBRARIES` through `LSEnvironment` in Steam.app's `Info.plist` and deploys a dylib into the bundle (`app/Sources/NotProtonApp/Model/SteamBundle.swift`, `SteamInstaller.swift`, `SteamModifications.swift`, `UpdateBlock.swift`); the dylib waits for `steamclient.dylib`, finds its targets by signature per Steam build (`dylib/resolver/`, a signature database) and hooks them (`dylib/hooks/`, `dylib/feats/compat*.c`, `nativeplatform.c`, `webpatch.c`). Its README pins Steam builds (stable 1794424036 when read; the current stable Macron knows is 1794536281) and says the Compatibility settings and the per-game "Force the use of a specific Steam Play compatibility tool" then work as on Linux. It is built around CrossOver; Macron needs only the switch, with Macron as the compatibility tool.
- Not yet checked on a Mac: whether the inner `steam_osx` accepts an inserted library given its signature flags and entitlements, and what changing `Info.plist` does to the bundle's signature, Gatekeeper and Steam's self-update. Check on a sandbox before designing around it.

## What Thalles decided, so you do not need to ask

- Modifying the running Steam client to switch Steam Play on is wanted. Keep it the smallest change that gives the experience, only on Steam builds Macron recognises, fully undone by `macron uninstall`, with Mac-native games untouched. GOAL.md gate 2 lists these rules.
- NotProton is the reference. Read it freely. Prefer Macron's own code; if taking NotProton's code is the practical way, it goes in its own folder (`steamplay/`) as its own binary under GPL-3.0 with their notices, as GOAL.md's licence rule says. Nothing GPL goes into `macron` itself.
- The Steam test account exists: `~/.config/macron/steam-test-account` on the dev machine (mode 600, `source`-able: `STEAM_TEST_USER`, `STEAM_TEST_PASSWORD`, `STEAM_TEST_EMAIL`). Never print it, never put it on a command line a workflow logs, in the repo, an artifact or a screenshot. If a login asks for a Steam Guard code or shows a CAPTCHA, stop that login, do not retry in a loop, and put it under "Needs Thalles"; keep going on everything that works against Steam's login window.
- His MacBook is off limits. Everything runs on the sbx sandboxes and in CI.
- Gate 4 (the Steam bridge) is unparked since 2026-11-15: do what NotProton does (GOAL.md, licence rule). It comes after gates 2 and 3. The port does not change the Steam client, so an Opus 5.5 subagent in its own worktree may take it.

## What done looks like

GOAL.md has the full text; do not call a gate green on a stand-in.

1. **Gate 2.** On a sandbox, Steam from Valve's DMG in /Applications, logged in to the test account, Macron installed: a Windows-only free game (Ultimate Custom Night, 871720) shows a working Install button, Steam downloads it into its normal library, Play starts it through `macron`, Steam shows it running, quitting returns to Play. Driven through Steam's real window, with a screenshot of each step, plus a Mac-native free game still installing and starting as before, and a clean refusal on an unknown Steam build.
2. **Gate 3.** The DMG, built by the Release workflow with the switch in it, published as a GitHub release of ThallesP/macron with release notes that say how to open an unsigned installer and how to uninstall; then downloaded from the release onto a fresh sandbox and taken through gate 2's flow and an uninstall that leaves Steam as it was. Publish as soon as this holds and put the link at the top of PROGRESS.md: he is waiting to try it.
3. **Then:** Ultimate Custom Night's frame rate started with Play on the sandbox (the `show-fps` switch exists), with the cause analysis PROGRESS.md says is still open; and the Overwatch attempt as gate 5 describes.

## How to work

- Thalles is away; do not stop to ask, and do not end your turn while work remains. Decisions only he can make go under "Needs Thalles" in PROGRESS.md while you work on something else.
- Do the Steam-client work yourself: do not delegate it, or a brief describing it, to Opus or Fable agents, whose filter stops on it. Work that does not touch the Steam client (CI plumbing, measurements, docs) may go to Opus 5.5 subagents, a few at a time; never Fable.
- Never weaken a check. Write tests. Stop every sandbox you start.
- Every step ends green and pushed to `main`; end commit messages with `Co-Authored-By: Claude Mythos 5.2 <noreply@anthropic.com>`.
- Keep PROGRESS.md and README.md true, including what a virtual GPU cannot show.
