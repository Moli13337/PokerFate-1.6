# Poker Fate - Private Server Client Patcher

Client-side patch toolkit for running Poker Fate against a private server.

## What This Does

The patches redirect the Poker Fate client to talk to a private server instead of the official servers:

- **Constants.lua URL redirect** — points the game at `http://127.0.0.1:8888/` for API traffic and `ws://127.0.0.1:9012` for WebSocket.
- **settings.json** — sets `m_DisableCatalogUpdateOnStart: true` so Addressables doesn't overwrite the local catalog.
- **steam_appid.txt** — appid `480` (Spacewar) so Steam SDK init succeeds without a real Steam client.

## Quick Start (Recommended)

Use the pre-patched files in `patched_files/`. Copy each file to its destination in your Poker Fate install:

| Source | Destination |
|---|---|
| `patched_files/steam_appid.txt` | `<game>/steam_appid.txt` |
| `patched_files/settings.json` | `<game>/Poker Fate_Data/StreamingAssets/aa/settings.json` |
| `patched_files/app_91421dd8f87b4ee559c3e3d77c2271f5.bundle` | `<game>/Poker Fate_Data/StreamingAssets/aa/StandaloneWindows64/LocalFiles/remote_res/gameres_assets_src/app_91421dd8f87b4ee559c3e3d77c2271f5.bundle` |

> If `LocalFiles/remote_res/gameres_assets_src/` does not exist under `StandaloneWindows64`, create it. The game also looks in the persistent data path (`%USERPROFILE%/AppData/LocalLow/Poker Fate/Poker Fate/remote_res/gameres_assets_src/`) — copy the bundle there if the StreamingAssets path is not picked up.

After copying, launch the game. It should connect to `127.0.0.1:8888`. Ensure the private server is running first.

## Re-generating Patches from Source (Advanced)

If you need to regenerate the patched bundles (e.g. the Constants.lua URL changed), use the Python scripts:

1. Install Python 3.10+ and dependencies:
   ```bash
   pip install -r lib/requirements.txt
   ```
2. Edit the hardcoded paths at the top of `patch_new_ver.py` (`new_ver`, `game_remote`, `persistent_path`) to match your environment.
3. Run:
   ```bash
   python patch_new_ver.py
   ```

The other scripts in this package are kept for reference:
- `patch_bundle.py` — single-bundle Constants.lua URL redirect (subset of `patch_new_ver.py`).
- `patch_game_v2.py` — alternate patcher that injects URL overrides into `main.lua` / `init.lua` and stubs SDK calls. Use only if `patch_new_ver.py` does not work for your game version.

## How It Works

The game stores Lua scripts XXTEA-encrypted inside Unity Addressables bundles. The patch scripts:

1. Load the Unity bundle via [UnityPy](https://github.com/K0lb3/UnityPy)
2. Decrypt the Lua script (XXTEA, key and delta in [docs/XXTEA.md](docs/XXTEA.md))
3. Replace the server URL constants (`http_host`, `ws_host`)
4. Re-encrypt and repack the bundle

The `settings.json` patch disables Addressables catalog updates so the local catalog is the only source of truth, preventing the game from overwriting patched bundles.

## Server Companion

This package only patches the **client**. The matching server is published separately as `poker-fate-server`. See that repository for setup instructions.

## Disclaimer

For educational and research purposes only. Do not use this for commercial purposes or to gain an unfair advantage on live services. All trademarks belong to their respective owners.
