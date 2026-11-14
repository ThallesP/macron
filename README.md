# Macron

Mac + Proton. Press Play on a Windows-only game in the Mac Steam client and have it run.

**Status:** nothing works yet. This is the plan.

## How it will work

1. **Steam hook**: switches on the Steam Play code that already ships (disabled) in Steam for Mac and registers Macron as a compatibility tool.
2. **Launcher**: the headless program Steam calls on Play. It prepares a private Windows environment for the game and starts it under Wine.
3. **Steam bridge**: a Mac port of Proton's `lsteamclient`, so games reach the native Steam client for ownership, achievements and cloud saves.
4. **Runtime**: Wine with DXMT (DirectX 11) and D3DMetal (DirectX 12). Rosetta 2 first, FEX later.
5. **Fixes database**: per-game settings keyed by Steam app ID.
6. **Installer**: one package that sets everything up. The only UI.

## Milestones

- [ ] Steam calls a dummy compatibility tool when Play is pressed
- [ ] One DirectX 11 game installs, launches, saves and unlocks an achievement
- [ ] Installer, fixes database and ten tested games
- [ ] DirectX 12, controllers, FEX backend

## Mac sandboxes (`sbx`)

Macron is developed on Linux, so `sbx` gives you a Mac to run commands on. Each sandbox is a GitHub Actions Mac runner (macOS 26, Apple silicon, 5 cores, 14 GB RAM, a logged-in desktop) that you reach over SSH.

Build it once. It needs Go 1.24+, OpenSSH, and a `gh` login that can start workflows on this repository:

```sh
cd tools/sandbox && go build -o ~/.local/bin/sbx ./cmd/sbx
```

```sh
id=$(sbx new --json | jq -r 'select(.event == "ready") | .id')   # ready in about 20 seconds
sbx exec "$id" -- sw_vers              # exits with the command's exit code
sbx exec "$id" 'cd /tmp && ls | wc -l' # one argument is a shell line
sbx scp ./build.tgz "$id":/tmp/build.tgz
sbx scp -r "$id":/tmp/out ./out
sbx ssh "$id"                          # interactive shell
sbx list                               # running sandboxes; --all adds stopped ones
sbx info "$id"
sbx extend "$id" --ttl 2h              # stop 2 hours from now instead
sbx stop "$id"
```

Things to know:

- A sandbox stops itself 30 minutes after `new` unless you pass `--ttl` or call `extend`. The timer does not look at activity. No sandbox runs longer than 5 hours 30 minutes.
- Stopping erases the machine. There are no snapshots yet.
- At most 5 sandboxes run at once (GitHub's limit for Mac jobs).
- Only the machine that ran `sbx new` holds the sandbox's SSH key, in `~/.local/state/sbx/`. Other machines can list and stop it, not log in.
- Every command accepts `--json`. `sbx`'s own failures exit 125 with an error code on stderr; wrong usage exits 2.
- The repository is public, so the workflow's inputs, logs and artifacts are too. `sbx` puts only public keys there, and the tunnel address encrypted. Never print a secret from a workflow step.

## Not planned

Games that need kernel anti-cheat (EasyAntiCheat, BattlEye, Vanguard).
