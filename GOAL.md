# Goal

Macron is Proton for Mac: a Mac user presses Play on a Windows-only game in the Mac Steam client and it runs. README.md has the architecture.

Phase 1 (2026-11-14) built the launcher, the pinned runtime, the fixes database and the scenario checks; its gates are at the end of this file and stay green.

Phase 2 is this file: make it fast, make Steam's own Install and Play buttons work, and ship something Thalles can install.

## The brief

Thalles, 2026-11-14, after trying phase 1 on his MacBook. Dictated; cleaned up, with the readings of garbled words marked:

> It's running, though super slow, and clearly we did not have the experience I wanted.
>
> Move this up to the dev machine and let it rip in there. Feel free for the dev machine to ask about anything that can only be investigated on this PC [the MacBook], but I think it makes sense to use GitHub Actions to investigate what's happening.
>
> See why it's so slow on Ultimate Custom Night. The game runs everywhere; it should run on an Intel Celeron without dropping under 40 FPS [reading of "the Tel Cellar ROM without 40 FPS"]. Investigate what's so slow and get the experience done.
>
> When it's done, get me the DMG or whatever I have to install so I can have the experience I wanted, because I didn't have that experience. You had to do some hacky workarounds under the hood to get this working.

"The experience" is his from the start: open the native Mac Steam client, press Install and then Play on a Windows-only game, and it runs. Macron is an installer that plugs into Steam with little or no UI of its own.

## What happened on the MacBook

Read this before planning; PROGRESS.md has the detail.

- Macron's registration alone does not switch on Steam Play. With `macron steam enable` applied to his logged-in Steam (build 1794536281), Steam logged `Mapping AppID 0 to tool "macron"`, but the Install button of a Windows-only game stayed greyed out and `steam://install/871720` answered **"An error occurred while installing Ultimate Custom Night: Invalid platform"**. Phase 1 parked exactly this question; it is now answered.
- The game only ran through a workaround: a logged-in Steam accepts `steam_osx +download_depot <app> <depot>` and downloads the Windows files, which were then started by hand with `SteamAppId=871720 macron waitforexitandrun`. That is the "hacky workaround"; it is not the product.
- Ultimate Custom Night (Steam app 871720, free, one 262 MB 32-bit Clickteam Fusion exe) reached its title screen and was "super slow". It loads `mmf2d3d9.dll`, so it draws with Direct3D 9; with the default profile (`dxmt`) that went to Wine's builtin `d3d9` → `wined3d` → `opengl32` (Apple's OpenGL, through Wine's 32-on-64 mode, under Rosetta). The window was full screen at 1920x1243 points on a Retina display. Wine's server in this build has no msync or esync. None of these has been measured yet; they are suspects, not a diagnosis.
- DXMT presents on a real Apple GPU with the patched `winemac.so`: the Direct3D 11 probe drew its picture on the M3 (60 frames in 529 ms) through `macron waitforexitandrun` with the default profile.

## Done means

Gates, in order of importance. The goal is complete when all four are green and the phase 1 gates still are.

1. **Fast.** Ultimate Custom Night runs at its own frame rate (it is a 60 fps game) on the MacBook through Macron, from its title screen into a night, measured and not judged by eye. The cause of the slowness is found by measurement (which layer costs the time: Direct3D translation, the 32-bit mode, Rosetta, the Wine server, the window or Retina path, sound) and written down with the numbers before and after. The fix belongs in Macron's defaults or runtime, so a game with no profile gets it too; a profile for 871720 alone is not enough if the cause is general (every Direct3D 9 game, every 32-bit game). A frame-rate check joins the scenario checks so this cannot regress silently: a legally free game or a program of ours that uses the same path (32-bit, Direct3D 9), with a frame-time measure and a threshold set from the Windows runner's numbers and the MacBook's, not from Macron's output alone.

2. **Steam's own buttons.** On the MacBook, with Macron installed and Thalles's logged-in Steam: a Windows-only game he owns shows a working Install button, Steam downloads it into its normal library folder through its normal UI, Play starts it through `macron` (Steam's `compat_log.txt` and macron's own log both show it), Steam shows the game as running, and quitting the game returns Steam to "Play". No terminal, no `download_depot`, no hand-set environment. If the Mac client needs its Steam Play switched on in memory, do that; Thalles has asked for this experience knowing Phase 1's registration was not enough. Rules for it:
   - Prefer the least invasive way that works, and say what was tried. Changing Valve's files on disk is the last resort; whatever is done must be undone completely by Macron's uninstall, and Steam must start normally afterwards.
   - Only act on Steam builds Macron recognises (the hash table), and refuse cleanly on others, as the hook does now. Steam updates itself often, so say how a new build gets supported and make that as automatic as it can be.
   - Mac-native games must keep working exactly as before.
   - CI keeps checking what it can without an account (applies to the current build, Steam still reaches its login window, refuses unknown builds).

3. **Something to install.** A DMG (or pkg inside one) published as a GitHub release of this repository: Thalles downloads it, installs it, opens Steam, and gate 2's experience is there. It bundles `macron` and fetches or carries the runtime, sets up the Steam side, and has an uninstall that puts everything back. No other UI is needed. Without an Apple Developer ID it cannot be notarised: make the unsigned path as painless as it can be, write the exact steps in the release notes, and put "a Developer ID" under "Needs Thalles". Test the published DMG itself on the MacBook from a clean state (install, gate 2's flow, uninstall), then leave the MacBook clean so Thalles installs it himself.

4. **The Steam bridge.** Games that talk to Steam find it: `SteamAPI_Init` succeeds inside a game started by Play, through a Mac port of Proton's `lsteamclient` (from Valve's Proton source, see the licence rule), so games that quit without Steam run, and achievements and Steam Cloud work for at least one game. Checked on the MacBook with a game Thalles owns that uses them.

Publish a release as soon as gate 3 is reached, even before gate 4, and say so at the top of PROGRESS.md: Thalles wants to try it. Publish again when gate 4 lands.

## Rules

- **PROGRESS.md is the loop's memory.** Read it first. Update it last: what was done, the evidence, what's next, what's blocked. Keep README.md true.
- **Thalles is away.** Don't stop to ask. A decision only he can make goes under "Needs Thalles" in PROGRESS.md, and you work on something else meanwhile.
- **Done means he would say so.** Phase 1 passed every gate and still missed what he wanted, because no gate pressed Play in a real Steam. Do not call a gate green on a stand-in for the real thing: gate 1 needs the real game at the real frame rate, gate 2 needs the real buttons in a logged-in Steam.
- **Never weaken a check to make it pass.** No raised thresholds, no dropped negative controls, no references regenerated from Macron's own output, no scenario marked as passing on partial evidence. Changing a check is fine only when the check itself was wrong; say why in the commit and in PROGRESS.md.
- **Models and usage.** Subagents run on Opus 5.5, never Fable. Run a few at a time, each with its own git worktree and a narrow brief. Use a workflow or Ultracode only when a problem clearly needs many parallel attempts, and note why in PROGRESS.md. On a usage-limit error: stop launching agents, write the state to PROGRESS.md, wait for the reset, carry on.
- **Games.** Legal downloads only: free ones in CI, and on the MacBook the games Thalles owns, downloaded by his own Steam. No cracks, no DRM bypass. Never commit game files, never upload them as artifacts, never copy them off the MacBook. A screenshot of a game's window as evidence is fine.
- **Licences.** Macron still has no licence. Record each dependency's licence in README.md. Changes to Wine are LGPL and get published. Don't copy code from GPL projects (NotProton, Proton-ish, Whisky); reading them to learn how something works is fine. Valve's Proton is the source for the Steam bridge; check the licence of each part you take and record it.
- **The repository is public.** Logs, artifacts and caches are world-readable and anyone can open a pull request. Never print a secret. No workflow may run code from a fork with access to anything: no `pull_request_target`, no self-hosted runners. Nothing from Thalles's Steam (account name, id, library, session files) goes into the repository, a log or an artifact.
- **Sandboxes.** At most 5 Mac jobs run at once. Stop every sandbox when its work is done and leave no runs in progress. The tunnel sometimes stalls (see README.md), so run long jobs detached on the sandbox and poll them, and let CI workflows be the standing check.
- **Every step ends green and pushed.** Tests pass, it's committed and pushed to `main`, CI passes. Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Where to investigate

Thalles asked for GitHub Actions to carry the investigation, with the MacBook for what only it can answer.

- **GitHub's Mac runners** (`sbx`, the workflows): everything that does not need a real GPU or his account. Reproducing the Direct3D 9 path with a free game on the same engine or a program of ours, profiling where the time goes (`sample`, Instruments' command line tools, Wine's debug channels), comparing backends and Wine builds, building Wine, studying the Steam client without logging in. Their GPU is virtual (no Metal 3, no hardware OpenGL), so frame rates there show relative cost, not the real number.
- **The MacBook**: real frame rates on a real Apple GPU, and anything that needs a logged-in Steam. Ultimate Custom Night cannot be downloaded without an account, so its runs happen here.

## The MacBook

The dev machine has a T3 link to "Thalles's MacBook Air" (environment `87511d05-cf0b-4b25-9d13-83efe54da30d`, access `auto`): M3, 16 GB, macOS 26.5, the only real Apple GPU here. Reach it with `delegate_task` and `target.environmentId`, with a brief that carries these rules. Thalles offered it and uses it at the same time, so behave as a guest who was lent a desk.

- **It can go offline at any time.** A `caffeinate -s` keeps it awake on AC power until about 2026-11-17 01:30 (-03); start another if it has ended and the machine is still needed. If it is unreachable, work on what the runners can do and try again later.
- **Thalles's Steam may be used for gates 1, 2, 3 and 4**, which is new in phase 2: he asked for this experience on this machine. He signed in on 2026-11-14 and the client updated itself to build 1794536281. Use Ultimate Custom Night (871720) first; other small Windows-only games he owns are fine if they have no anti-cheat and no online play (Skyraine, 679430). **Never start a multiplayer or anti-cheat game** (Counter-Strike 2, PUBG, Paladins, Marvel Rivals, Dead by Daylight, and the like), with or without Macron: a modified client next to anti-cheat is a risk to his account. Don't buy, add, remove or change anything in his account, store, friends, chat or community, and don't read more of his library than the test needs.
- **Every session ends clean.** Back up `config/config.vdf` before touching Steam. When the session ends: quit what you started, uninstall Macron completely (the Steam side, `~/Library/Application Support/Macron`, any app, any launch agent), check Steam starts and reaches the library normally, quit it if it was not running when you came, and remove your working folder (`~/macron-test`). Never delete anything you didn't create. Games installed through Steam's own Install button during a test are uninstalled through Steam too.
- **He is using the machine.** A game opens over his work. Keep sessions short and batched, run windowed when the game allows it, and never leave a game or a dialog on his screen.
- **Screenshots of one window only**: `screencapture -l <window id>` of the game's or Steam's window. Never the whole screen, which shows his other apps. Delete the pictures when the session ends unless they are evidence of a game's window.
- **Disk:** 24 GB was free on 2026-11-14. Check `df` first and keep Macron's footprint under 6 GB.
- Don't touch his browser or his files, and don't change system settings.

## When stuck

- If a check fails for a reason outside Macron (a flaky runner, a tool bug, a download that moved), fix the tool, write it down in PROGRESS.md, and carry on.
- If the cause of the slowness is not where the first measurement points, measure the next layer; don't ship a guess. If the fix needs a different Wine build (one with msync, CrossOver's sources, a newer DXVK or MoltenVK), build and publish it the way `runtime/wine/` does.
- If switching on Steam Play resists one approach, write down what was tried and what the client did, and try the next. How NotProton does it is readable; the code is not copyable.
- If a decision only Thalles can make blocks the way, add it under "Needs Thalles" and work on another gate.

## Phase 1 gates (green, keep them green)

1. One command runs a scenario end to end in a Mac sandbox and in CI (`macron-check sandbox <scenario>`).
2. A real game reaches its menu and matches the original on Windows (OpenTTD).
3. Scenarios launch through `macron` with a pinned runtime, a per-game prefix and the fixes database.
4. Three games from three engines pass, one of at least 1 GB (OpenTTD, Xonotic, SuperTuxKart).
5. The Steam registration applies to the current build, Steam still reaches its login window, and unknown builds are refused.
