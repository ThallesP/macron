# Game survey

Candidates for gates 2 and 4: real Windows games, free and legal to download, that reach their main menu on a GitHub `windows-latest` runner (the reference) and under Wine on a GitHub `macos-26` runner (virtual M2 Pro, paravirtual GPU). Surveyed 2026-11-14 on the `game-survey` branch with throwaway scripts (`ci/game-survey/`, `.github/workflows/game-survey.yml`). Menu screenshots, cropped to the game window and halved, are in `docs/game-survey/`.

## What the runners can do

Each of these is the mechanism behind a result below.

- **The Mac runner has no hardware OpenGL.** A native CGL probe on the sandbox gets `invalid pixel format` for every accelerated pixel format. Without `kCGLPFAAccelerated` it gets "Apple Software Renderer" (GL 2.1 legacy, 4.1 core), both for arm64 and under Rosetta.
- **Stock Wine can't use that software renderer.** winemac's `init_context` (`dlls/winemac.drv/opengl.c`, Wine 11.21) always asks for `kCGLPFAAccelerated`, so wgl setup fails before any pixel format exists: `err:wgl:internal_context_create Failed to create internal root context`, and then the game reports `No matching GL pixel format available`. The `HKCU\Software\Wine\Mac Driver\AllowSoftwareRendering=y` registry value doesn't help, because it only affects the later pixel-format enumeration. Checked: the value was set and the error stayed the same.
- **Mesa llvmpipe works under Wine.** Mesa's `opengl32.dll` + `libgallium_wgl.dll` (pal1000/mesa-dist-win **26.1.8** MSVC, x64) next to the exe, with `WINEDLLOVERRIDES=opengl32=n,b` and `GALLIUM_DRIVER=llvmpipe`, renders in software and presents through GDI, with no host OpenGL involved. Xonotic logs `GL_RENDERER: llvmpipe (LLVM 23.1.1, 128 bits)`, `GL_VERSION: 4.6 (Compatibility Profile) Mesa 26.1.8`. It's the same renderer as the Windows reference, so the two pictures should agree closely.
- **Mesa 26.2.6's MSVC package has no `x64/opengl32.dll` or `libgallium_wgl.dll`** (its x64 folder holds only Vulkan, OpenCL and d3d10warp files; the MinGW package has them but needs the Vulkan runtime). Main's `internal/check/extras.go` pins 26.2.6, so `mesa-llvmpipe` will fail with "none of [opengl32.dll ...]". Use 26.1.8: `https://github.com/pal1000/mesa-dist-win/releases/download/26.1.8/mesa3d-26.1.8-release-msvc.7z`, SHA-256 `4c6d32e653e0ff9ad07796e40c0bcfabf2764d849e3ce4f3b1590112c87e42f9`, 71005136 bytes. `libglapi.dll` is gone since Mesa 25.0, which is fine.
- **On the Windows runner** (Windows Server 2025, Microsoft Basic Render Driver, 1024x768): Direct3D 9/11 games run on WARP. OpenGL games need the Mesa DLLs next to the exe (`GALLIUM_DRIVER=llvmpipe`). GDI's own OpenGL 1.1 is useless. A Windows Terminal window (the runner agent) sits behind every game; capture the window, not the desktop.
- **DXVK-macOS 1.10.3 over MoltenVK** creates a Direct3D 11 device at feature level 11.0 on the runner (osu! and UT99), but no tested game drew a visible picture with it. **DXVK's Direct3D 9 can't create a device at all**: MoltenVK reports `vkCreateDevice(): Requested physical device feature specified by the 5th flag in VkPhysicalDeviceFeatures is not available` and the 39th, which are `geometryShader` and `shaderCullDistance`, then `err: DxvkAdapter: Failed to create device` (SuperTuxKart and Rigs of Rods). The parent found DXMT can't present on stock Wine.
- **wined3d's Vulkan renderer works for Direct3D 9.** `WINE_D3D_CONFIG=renderer=vulkan` in the plain Wine tree (Gcenx's build ships MoltenVK 1.4.0) draws SuperTuxKart's D3D9 menu on the runner, matching Windows. wined3d's default GL renderer fails there like everything else on GL: `err:d3d:wined3d_caps_gl_ctx_create Failed to find a suitable pixel format`.
- **macOS asks "Allow … to find devices on local networks?"** when a game scans the LAN (Xonotic's server list). The dialog floats over the desktop; `screencapture -l` of the game window leaves it out.

## Recommendation

Three games, three engines, one of them at least 1 GB. All three reach their main menu on both runners.

1. **OpenTTD 15.3** (OpenTTD's own engine, GDI): already passing on main (the parent's work).
2. **Xonotic 0.8.6** (DarkPlaces, OpenGL, **1.24 GB**): menu on Windows with Mesa llvmpipe and on the Mac runner with Wine + Mesa llvmpipe's `opengl32.dll` next to the exe. The parent has taken it. It needs the Mesa-next-to-the-exe launcher feature below.
3. **SuperTuxKart 1.5 in Direct3D 9 mode** (Antarctica, STK's Irrlicht fork, 922 MB): the non-OpenGL game that passes through an existing backend. Main menu on Windows (WARP) and on the Mac runner through `wined3d` with `WINE_D3D_CONFIG=renderer=vulkan`, with the same picture on both. No popup, no Mesa, no new launcher feature: `fixes/games/supertuxkart.json` + `scenarios/supertuxkart/scenario.json` on this branch.

Next in line:

- **Unreal Tournament GOTY + OldUnreal 469f** (Unreal Engine 1, 759 MB) reaches its static title screen ("Press ESC to begin.") on both runners with its default `OpenGLDrv` and Mesa llvmpipe. It's the fourth engine, but it's one key press short of the menu, and it needs two launcher features: Mesa next to the exe, and a one-time `UCC decompress` of three maps. Its `D3D11Drv` works on Windows but stays black under DXVK, and `D3D9Drv` is black under wined3d-Vulkan. Drafts: `fixes/games/ut99.json`, `scenarios/ut99/scenario.json`.
- **Endless Sky 0.11.4** (own engine, OpenGL) reaches its menu on both with Mesa, the same as Xonotic.

Rigs of Rods (OGRE, Direct3D 9 only on Windows) reaches its menu on Windows but not on the Mac (details below). No Direct3D 11 game drew anything visible through `dxvk` on the runner.

**Launcher feature the OpenGL games need:** put `opengl32.dll` + `libgallium_wgl.dll` from Mesa 26.1.8 MSVC `x64/` next to the exe in the Mac prefix, with `opengl32=n,b` and `GALLIUM_DRIVER=llvmpipe`, the way `windowsExtra: ["mesa-llvmpipe"]` does on Windows. Without it, OpenGL games fail on the runner the same way plain wined3d does.

## Results

"Mac" is the sandbox runner (Wine 11.21 from Gcenx, x86_64 under Rosetta). Every result was checked by looking at the screenshot, not inferred from the process staying up.

| Game | Windows runner | Mac runner | Backend on the Mac | What failed |
|---|---|---|---|---|
| Xonotic 0.8.6 | menu | menu | Wine + Mesa llvmpipe opengl32 | plain winemac GL: `Failed to set video mode to 800x600: No matching GL pixel format available` → `Quake Error: Video modes failed` |
| SuperTuxKart 1.5 (OpenGL) | menu | menu | Wine + Mesa llvmpipe opengl32 | plain winemac GL: `[IrrDriver Logger]: Could not initialize display!` ×4 → `[fatal] irr_driver: Couldn't initialise irrlicht device` |
| **SuperTuxKart 1.5 (`--render-driver=directx9`)** | **menu** | **menu, same picture** | **wined3d, `WINE_D3D_CONFIG=renderer=vulkan`** | both fall back to the fixed pipeline (`Could not load shader function D3DXAssembleShader from dll, shaders disabled: d3dx9_43.dll`), which shows an "old graphics driver" popup unless `<Video old_driver_popup="false"/>` is set. DXVK d3d9: `DxvkAdapter: Failed to create device` (missing geometryShader/shaderCullDistance) → `[fatal] irr_driver: Couldn't initialise irrlicht device` |
| OpenTTD 15.3 | menu | menu | GDI (no GPU) | Windows with the default hardware acceleration: "no compatible GPU found" popup; fixed with `video_hw_accel = false` |
| Endless Sky 0.11.4 | menu | menu | Wine + Mesa llvmpipe opengl32 | plain winemac GL: `Unable to create window! (SDL message: "No matching GL pixel format available")` |
| 0 A.D. 0.28.0 | menu | crash before the menu | Wine + Mesa llvmpipe opengl32 | GL comes up (crashlog: `Video Card: Mesa llvmpipe (LLVM 23.1.1, 128 bits)`, `Video Driver: 4.6 (Compatibility Profile) Mesa 26.1.8`, `Video Mode: 800x600:24`), then `unhandled exception (Access violation reading 0x00000080)`, `Location: unknown:0`, no call stack. The NSIS install under Wine also logs `err:virtual:allocate_virtual_memory out of memory for allocation ... size 82001000` but finishes. Logs are in `%LOCALAPPDATA%\0ad\logs` |
| osu!(lazer) 2026.1005.0 | logo screen (the menu shows only after a click) | crash | DXVK-macOS | D3D11 comes up, then osu!'s own auth module crashes: `osu.Game.Auth.Updater` downloads and loads a native module, then `err:seh:NtRaiseException Unhandled exception code c0000005`, exit code 5. With `mscoree` disabled it fails earlier: `fixup_imports_ilonly mscoree.dll not found` |
| Unreal Tournament GOTY + 469f, `OpenGLDrv` (default) | title screen "Press ESC to begin." | title screen, same picture | Wine + Mesa llvmpipe opengl32 | (works) |
| Unreal Tournament, `D3D11Drv` | title screen | black window | DXVK-macOS | `D3D11CoreCreateDevice: Using feature level D3D_FEATURE_LEVEL_11_0`, swap chain made, `D3D11Drv: Resizing scene buffers to 800 x 600`, then about 190,000 `[mvk-warn] VK_ERROR_OUT_OF_POOL_MEMORY: VkDescriptorPool exhausted ... Allocating descriptor dynamically` and nothing on screen in 240 s |
| Unreal Tournament, `D3D9Drv` | not tried | black window | wined3d, renderer=vulkan | `Enter D3D9::SetRes(800, 600, 4, 0)` is the last line of UnrealTournament.log |
| Rigs of Rods 2026.01 (OGRE, D3D9) | menu | background only, no menu | wined3d, renderer=vulkan | `Created D3D9 Rendering Window ... 800x600`, background drawn, RoR.log stops at `Texture 'accept.png': Loading` and the menu never appears (240 s). DXVK d3d9: `Ogre::RenderingAPIException: Cannot create device! in D3D9Device::createD3D9Device`, then a crash. The silent install under Wine finishes, but something after it page-faults into winedbg (`Unhandled page fault on read access to 00000000`); kill it once `unins000.exe` exists |
| Trackmania Nations Forever | stuck at the first-run "TmForever Network check" dialog | not tried | | first-run dialogs; no known switch to skip them |

## Candidates, ranked

Sizes are bytes. Every SHA-256 was computed here from the pinned URL. Windows extraction uses the runner's `C:\Program Files\7-Zip\7z.exe`, macOS uses `7zz` (`brew install sevenzip`) or `unzip`/`ditto`. Window size is 800x600 everywhere because the runner desktop is 1024x768 and a 1024x768 window doesn't fit with its title bar.

### 1. Xonotic 0.8.6 (taken by the parent)

- **Engine / API:** DarkPlaces, OpenGL through SDL2. `xonotic.exe` (64-bit SDL) is what the launcher starts; `xonotic-wgl.exe` also ships.
- **Download:** `https://dl.xonotic.org/xonotic-0.8.6.zip` (official site, versioned file name)
- **SHA-256:** `50850f8d800e7499722f6ea61e478e96464a375494b5a24da93aa0598cbe964d`, size 1238439495 (1.24 GB)
- **Licence:** GPLv3+ code, free-licensed data: <https://gitlab.com/xonotic/xonotic/-/raw/master/COPYING>
- **Archive:** zip; `7z x` or `unzip`. Top folder `Xonotic/`.
- **Exe:** `Xonotic/xonotic.exe`, working directory `Xonotic`.
- **Windowed:** `-window -width 800 -height 600 +vid_fullscreen 0 +vid_width 800 +vid_height 600`.
- **No-input start:** no intro video. Two first-run dialogs, both skipped from the command line: the terms-of-service dialog (`+_termsofservice_accepted 9999`; it appears when the server reports a newer ToS version) and the player-name dialog (`+_cl_name Macron`; shown while `_cl_name` equals its default). `-nohome` keeps config and `-condebug`'s `qconsole.log` in `Xonotic/data/`. Menu up within 30 s on both runners.
- **Menu:** static background with the logo and four panels. The Multiplayer panel's server list fills from the internet and differs between runs: ignore that region (about x 0.545, y 0.258, w 0.36, h 0.37 of the window), or stop the master-server query (DarkPlaces `sv_master*`/`sv_masterextra*` cvars, not tried).
- **OCR text:** "Singleplayer", "Multiplayer", "Settings", "Credits", "Quit", "0.8.6".
- Screenshots: `game-survey/windows-xonotic.png`, `game-survey/mac-xonotic-mesa.png`.

### 2. SuperTuxKart 1.5 (Direct3D 9 mode recommended)

- **Engine / API:** Antarctica (STK's Irrlicht fork). OpenGL 3.1+ is the default (`render_driver` default `opengl`); `--render-driver=directx9` and `vulkan` also exist in the Windows x86_64 build. Recommended here: **Direct3D 9**, run as `wined3d` + `WINE_D3D_CONFIG=renderer=vulkan` on the Mac. It works on the runner with no Mesa.
- **Download:** `https://github.com/supertuxkart/stk-code/releases/download/1.5/SuperTuxKart-1.5-win.zip`
- **SHA-256:** `9df7e2d67e8562127a3bb633030f1bd4ee77fa9e7f0ae110897473025af8acdc`, size 922430727
- **Licence:** GPLv3 code, data under free licences: <https://raw.githubusercontent.com/supertuxkart/stk-code/master/COPYING>
- **Archive:** zip. Installers (`-installer-x86_64.exe`) also exist; use the zip.
- **Exe:** `SuperTuxKart-1.5-win/stk-code/build-x86_64/bin/supertuxkart.exe`, working directory `SuperTuxKart-1.5-win/stk-code` (it finds `data/` from there).
- **Windowed:** `--windowed --screensize=800x600` (plus `--render-driver=directx9`).
- **No-input start:** two first-run screens. Write `%APPDATA%\supertuxkart\config-0.10\players.xml` with a current player (skips the "create a player" screen) and `config.xml` with `<enable_internet value="2" />` (skips "allow internet access?"). In D3D9 mode both runners fall back to the fixed pipeline (no `d3dx9_43.dll` on Windows Server, and Wine's isn't used either), which shows a "Your graphics driver appears to be very old" popup. `<Video old_driver_popup="false" />` turns it off. Grouped settings are **attributes** of the group element; a child element gives "Unknown value for old_driver_popup". Files in `ci/game-survey/config/stk/`; the 18 remaining "Unknown value" log lines are STK complaining about attributes that aren't set, and are harmless.
- **Menu:** static. **OCR:** "Story Mode", "Singleplayer", "Splitscreen Multiplayer", "Online", "Addons", the player name "Macron".
- Screenshots: D3D9 `game-survey/windows-stk-d3d9.png` and `game-survey/mac-stk-d3d9-wined3d-vulkan.png` (same picture); OpenGL `game-survey/windows-stk.png` and `game-survey/mac-stk-mesa.png`.

### 3. OpenTTD 15.3 (taken by the parent)

- **Engine / API:** OpenTTD's own; `win32` video driver. Hardware acceleration (OpenGL) is the Windows default but is refused on both runners, so set `video_hw_accel = false` and both sides blit in software through GDI.
- **Downloads:** `https://cdn.openttd.org/openttd-releases/15.3/openttd-15.3-windows-win64.zip` (`61c0a6a43d81008c7ff4330fb56351bbf66a980cdad041f1d0e08b51f2eeb34c`, 11250350) + OpenGFX `https://cdn.openttd.org/opengfx-releases/8.0/opengfx-8.0-all.zip` (`43a0c1dabf39cb865394f3a6cc36d4da5c10ecfaaf55652043104806810903be`, 3583152) + OpenSFX `https://cdn.openttd.org/opensfx-releases/1.0.3/opensfx-1.0.3-all.zip` (`e0a218b7dd9438e701503b0f84c25a97c1c11b7c2f025323fb19d6db16ef3759`, 11566000), the last two unzipped into `baseset/` (each holds one .tar the game reads as is).
- **Licence:** GPLv2 (<https://raw.githubusercontent.com/OpenTTD/OpenTTD/master/COPYING.md>), OpenGFX GPLv2, OpenSFX CC BY-SA 3.0.
- **Start:** `openttd.exe -c openttd.cfg -r 800x600` with `[network] participate_survey = no`. Without OpenSFX a red "Only a fallback sound set was found" message stays up.
- **Menu:** the intro game animates behind a static menu window. The mouse sits at the screen centre on the Windows runner and hovers "Multiplayer" (tooltip); move it away before capturing.

### 4. Endless Sky 0.11.4

- **Engine / API:** own engine, OpenGL 3.3 core through SDL2.
- **Download:** `https://github.com/endless-sky/endless-sky/releases/download/v0.11.4/EndlessSky-win64-v0.11.4.zip`
- **SHA-256:** `d795bf08472089ade412fa3d1167bb6f0ee0cb553025a930328d635de6364ccc`, size 370965835
- **Licence:** GPLv3, assets mixed CC: <https://raw.githubusercontent.com/endless-sky/endless-sky/master/license.txt>
- **Archive:** zip with files at the top level. **Exe:** `Endless Sky.exe`.
- **Windowed:** fullscreen by default. Write `%APPDATA%\endless-sky\preferences.txt` with `"window size" 800 600` and `fullscreen 0`.
- **No-input start:** goes straight to the menu, "No Pilot Loaded".
- **Menu:** the starfield is static, but the credits scroll in the left panel. The layout is made for 1024x768 and is cut off at the sides at 800x600.
- **OCR:** "ENDLESS SKY", "Quit", "Manage Pilots...", "No Pilot Load(ed)".
- Screenshots: `game-survey/windows-endlesssky.png`, `game-survey/mac-endlesssky-mesa.png`.

### 5. 0 A.D. 0.28.0 (alternative ≥ 1 GB game)

- **Engine / API:** Pyrogenesis, OpenGL (`rendererbackend = "gl"`); Vulkan optional.
- **Download:** `https://releases.wildfiregames.com/0ad-0.28.0-win64.exe` (official; matches the published `.sha256sum`)
- **SHA-256:** `9b5e1a16d4e0df9c84e052f56fb75da69518d5634c88d2b38db69baee63c16a1`, size 1616727146 (1.62 GB)
- **Licence:** GPLv2+ code, CC BY-SA 3.0 art: <https://gitea.wildfiregames.com/0ad/0ad/raw/branch/main/LICENSE.md>
- **Archive:** NSIS installer. 7-Zip 26.00 (Linux) can't open it ("Is not archive"). The silent install `0ad-0.28.0-win64.exe /S /D=C:\path` works on Windows with no window (233 s on the runner). Main's scenarios can't unpack it yet.
- **Exe:** `binaries/system/pyrogenesis.exe`.
- **Config:** `%APPDATA%\0ad\config\user.cfg`: `windowed = "true"`, `xres = "800"`, `yres = "600"`, `gui.splashscreen.enable = "false"`, `gui.splashscreen.version = "251867d812acc3a2d732a2c7d532ccb0"`. That's the MD5 of `gui/splashscreen/splashscreen.xml` at tag v0.28.0; the splash shows whenever the stored value differs from it.
- **Menu:** the background art pans slowly. A "Help improve 0 A.D.!" box sits bottom right (not modal).
- **OCR:** "Learn to Play", "Single-player", "Multiplayer", "Settings", "Scenario Editor", "Exit", "Build: Feb 16, 2026 (release-0.28.0, a2cae)".
- Screenshot: `game-survey/windows-0ad.png`.

### 6. Unreal Tournament GOTY with OldUnreal 469f

- **Engine / API:** Unreal Engine 1. 469f ships `OpenGLDrv` (the default in `Default.ini`), `D3D9Drv`, `D3D11Drv`, `ICBINDx11Drv`, `VulkanDrv`, `XOpenGLDrv` and `SoftDrv`.
- **Downloads:** the original disc image `https://files.oldunreal.net/UT_GOTY_CD1.ISO` (`e184984ca88f001c5ddd52035d76cd64e266e26c74975161b5ed72366c74704f`, 649633792; same hash pinned in OldUnreal's installer, mirror `https://archive.org/download/ut-goty/UT_GOTY_CD1.iso`) + patch `https://github.com/OldUnreal/UnrealTournamentPatches/releases/download/v469f-rc6/OldUnreal-UTPatch469f-Windows-amd64.zip` (`b8de78a4dca46ec8d56fbcd4f1f4bdc41796a1c480f184308b1b1646e7129fdc`, 108225846). 469f is marked "release candidate" but not a GitHub prerelease.
- **Legal basis:** OldUnreal's own installers download this disc image from their file servers: <https://www.oldunreal.com/downloads/unrealtournament/full-game-installers/>, installer source <https://github.com/OldUnreal/FullGameInstallers>. I didn't find Epic's permission written on that page; it rests on OldUnreal's 2024 announcement and should be confirmed before relying on it.
- **Unpack:** `7z x` the ISO, then unzip the patch over it.
- **One-time step:** every map on the disc is compressed (`Maps/*.unr.uz`). The menu needs `CityIntro`, `Entry` and `UT-Logo-Map`: run `System\UCC.exe decompress ..\Maps\<map>.unr.uz` in `System\` and move the `.unr` it writes there into `Maps\`. Main's launcher has no step like this yet.
- **Config:** `System\UnrealTournament.ini` = `Default.ini` with `StartupFullscreen=False`, `WindowedViewportX=800`, `WindowedViewportY=600`, `FirstRun=469`, `LocalMap=UT-Logo-Map.unr` and the renderer (`GameRenderDevice=OpenGLDrv.OpenGLRenderDevice` is the default); start `System\UnrealTournament.exe -nohomedir`. The draft profile carries the whole ini as a file.
- **No-input start:** with the default `LocalMap=CityIntro.unr` it plays the animated city fly-by and shows no menu until Esc. With `UT-Logo-Map` it shows a static title screen: the logo and "Press ESC to begin." The menu itself needs one key press.
- **Results:** OpenGLDrv + Mesa llvmpipe gives the title screen on both runners, the same picture (`windows-ut99-gl.png`, `mac-ut99-gl-mesa.png`). D3D11Drv gives it on Windows but a black window under DXVK on the Mac. D3D9Drv is black under wined3d-Vulkan.
- **OCR:** "Press ESC to begin."

### 7. osu!(lazer) 2026.1005.0

- **Engine / API:** osu-framework (Veldrid). On Windows the "Automatic" renderer order is Direct3D 11, then Deferred Direct3D 11, then OpenGL (`GameHost.GetPreferredRenderersForCurrentPlatform`).
- **Download:** `https://github.com/ppy/osu/releases/download/2026.1005.0-lazer/osulazer-2026.1005.0-lazer-full.nupkg` (Velopack package, a zip)
- **SHA-256:** `d23d47d63ccb9c7d157b25cf5ac465d0e4d4cb17bab67c83f1ac112afd3e6cfa`, size 184409512
- **Licence:** code MIT (<https://raw.githubusercontent.com/ppy/osu/master/LICENCE>), resources CC BY-NC 4.0 (<https://raw.githubusercontent.com/ppy/osu-resources/master/LICENCE.md>).
- **Exe:** `lib/app/osu!.exe`. Config: `%APPDATA%\osu\framework.ini` (`WindowMode = Windowed`, `WindowedSize = 800x600`) and `game.ini` (`ShowFirstRunSetup = False`).
- **Why it's ranked low:** the idle main menu is only the big "osu!" logo (the buttons appear after a click), so there's little for OCR. On the Mac it crashes in osu!'s own native auth module after D3D11 is up (see the results table). It needs `mscoree` enabled; main's launcher disables it by default, but a profile can set `"mscoree": "b"`.

### 8. Rigs of Rods 2026.01

- **Engine / API:** OGRE 14. The source can build Direct3D 9, Direct3D 11 and GL render systems (`source/main/plugins.cfg.in`).
- **Download:** `https://github.com/RigsOfRods/rigs-of-rods/releases/download/2026.01/rigs-of-rods-2026.01.exe` (`5a66fbfe2a2bcf4e689c5d3ea23587a58feb3d8e151b23c16c1a997bd56a930c`, 158220929). Every release is an Inno Setup installer only. This one is Inno 6.6, which innoextract 1.9 can't read; `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP- /DIR=...` installs with no window.
- **Licence:** GPLv3: <https://raw.githubusercontent.com/RigsOfRods/rigs-of-rods/master/COPYING>
- **Renderer:** the Windows release ships only `RenderSystem_Direct3D9` (its log: `Loading library .\RenderSystem_Direct3D9`, `Auto-selected renderer plugin 'Direct3D9 Rendering Subsystem'`), so on Windows it's a Direct3D 9 game.
- **Exe / config:** `RoR.exe` in the install folder. Without `Documents\My Games\Rigs of Rods\config\ogre.cfg` it opens near-fullscreen at an auto-detected 1600x1200. With `Render System=Direct3D9 Rendering Subsystem` and, under `[Direct3D9 Rendering Subsystem]`, `Full Screen=No` and `Video Mode=800 x 600 @ 32-bit colour`, it opens an 800x600 window.
- **Menu (Windows):** "Main menu", "Single player", "Multiplayer", "Repository", "Settings", "Controls", "About", "Exit game", plus "Game version: 2026.01". **The background picture is chosen at random each start** (a truck in one run, a sports car in the next, a Tatra truck on the Mac), so the picture check would need it pinned. No setting for that was found. A "Cache updated" toast shows on first start.
- **Mac:** see the results table (DXVK d3d9 can't create a device; wined3d-Vulkan draws the background and then stalls before the menu). Screenshots: `windows-ror.png`, `mac-ror-wined3d-vulkan.png`.

### 9. Trackmania Nations Forever

- **Engine / API:** Nadeo GameBox, Direct3D 9 only.
- **Download:** `http://nadeo-download.cdn.ubi.com/trackmaniaforever/tmnationsforever_setup.exe` (Ubisoft's CDN, plain HTTP; `2f659138ed4409da404970841e18f03d29921beaf6a424824c8312ddb20f6355`, 530600781). Freeware from Ubisoft/Nadeo; the EULA is in the installer.
- **Archive:** a 7z wrapper around an Inno Setup 5.2.3 installer (`TmNationsForever_Setup_Tmp.exe` + `-1.bin`). `innoextract` reads the inner one. `/VERYSILENT /DIR=...` installs with no window.
- **Why it's ranked low:** `TmForever.exe` starts with a modal "TmForever Network check" dialog, and a profile and nickname setup follows. No config switch to skip them was found. It also wants d3dx9 DLLs.

### Not tested further

- **Stunt Rally 3.3** (OGRE-Next, 2.19 GB, `https://downloads.sourceforge.net/project/stuntrally/3.3/StuntRally-3.3-windows.zip`, `d27cf91b1a752f498c33dd72008804dad6eef90575f968901a1376deb42b8672`, 2189124075, GPLv3). The Windows build loads only `RenderSystem_GL3Plus` and `RenderSystem_Vulkan`; Direct3D 11 is commented out in `bin/Release/plugins_Windows.cfg`. The docs ask for a dedicated GPU with OpenGL 4 and say integrated GPUs struggle, and the zip holds an Inno installer split across `.bin` files. Too heavy for llvmpipe on both runners.
- **Red Eclipse 2.0.0** (Tesseract, OpenGL, `https://github.com/redeclipse/base/releases/download/v2.0.0/redeclipse_2.0.0_win.zip`, `baf9477e68359e622c8fff315d93eab703169aea7ad0b31708e84caf899371dc`, 967974793, zlib + CC BY-SA, <https://raw.githubusercontent.com/redeclipse/base/master/doc/license.txt>). Just under 1 GB; exe `redeclipse-2.0.0-win/bin/amd64/redeclipse.exe` (or `redeclipse.bat`). Should behave like Xonotic with Mesa; not run.
- **Unity freeware:** none found with a stable, pinnable direct URL (no GitHub release or official direct file) in the time available. The usual hosts are itch.io or store pages.
- Smaller engines considered: Luanti 5.18 (17 MB, an engine without a game), Teeworlds 0.7.5 (10 MB), OpenRA (needs content downloads on first run), GZDoom/UZDoom + Freedoom (OpenGL/Vulkan, an IWAD picker unless `-iwad` is given).

## Scripts

- `ci/game-survey/games.json`: one entry per game (downloads with SHA-256, unpack method, exe, args, config files, prelaunch steps, Mesa yes/no, Mac backend).
- `ci/game-survey/windows.ps1`: runs on `windows-latest`; screenshots via `System.Drawing.Graphics.CopyFromScreen`, artifact holds screenshots and the games' logs only.
- `ci/game-survey/mac.sh`: runs on an `sbx` sandbox, detached; `setup` installs Wine 11.21 + DXMT 0.80 (as in PROGRESS.md), `run <game>` makes a prefix per game, runs prelaunch steps (UT99's `UCC decompress`) and derived configs, adds Mesa or DXVK-macOS 1.10.3 (d3d9/d3d11/dxgi) as the game needs, applies `mac_env` (e.g. `WINE_D3D_CONFIG=renderer=vulkan`), launches, and runs `screencapture -x` at fixed times. Variants (`stk-d3d9-vk`, `ut99-gl`, `ror-vk`, …) differ only in renderer and backend.
