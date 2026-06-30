"""Poker Fate - New Version Binary Patcher
Patches hot-update bundles (Constants, SdkHelper, YiDunHelper) and GameAssembly.dll.
"""
import os
import sys
import struct
import shutil
from pathlib import Path

sys.path.insert(0, os.path.expanduser("~/.trae/python/lib"))
import UnityPy

XXTEA_KEY = "bee#happy&pkproject"
XXTEA_DELTA = 0x9E3779B9


def xxtea_encrypt(data, key_bytes):
    """XXTEA encrypt."""
    if len(data) < 8:
        return data
    # Pad to multiple of 4
    pad = (4 - len(data) % 4) % 4
    data = data + b'\x00' * pad
    n = len(data) // 4
    v = list(struct.unpack(f'<{n}I', data))
    k = list(struct.unpack('<4I', key_bytes[:16].ljust(16, b'\x00')))
    rounds = 6 + 52 // n
    total = 0
    for _ in range(rounds):
        total = (total + XXTEA_DELTA) & 0xFFFFFFFF
        e = (total >> 2) & 3
        for i in range(n):
            v_i = v[i]
            v_next = v[(i + 1) % n]
            v_prev = v[(i - 1) % n] if i > 0 else v[n - 1]
            v[i] = (v[i] + (((v_prev >> 5 ^ v_next << 2) + (v_next >> 3 ^ v_prev << 4)) ^ ((total ^ v_next) + (k[(i & 3) ^ e] ^ v_prev)))) & 0xFFFFFFFF
    return struct.pack(f'<{n}I', *v)


def xxtea_decrypt(data, key_bytes):
    """XXTEA decrypt."""
    if len(data) < 8:
        return data
    n = len(data) // 4
    v = list(struct.unpack(f'<{n}I', data[:n*4]))
    k = list(struct.unpack('<4I', key_bytes[:16].ljust(16, b'\x00')))
    rounds = 6 + 52 // n
    total = (rounds * XXTEA_DELTA) & 0xFFFFFFFF
    while total != 0:
        e = (total >> 2) & 3
        for i in range(n-1, 0, -1):
            z = v[i-1]
            v[i] = (v[i] - (((z >> 5 ^ v[(i+1) % n] << 2) + (v[(i+1) % n] >> 3 ^ z << 4)) ^ ((total ^ v[(i+1) % n]) + (k[(i & 3) ^ e] ^ z)))) & 0xFFFFFFFF
        z = v[n-1]
        v[0] = (v[0] - (((z >> 5 ^ v[1] << 2) + (v[1] >> 3 ^ z << 4)) ^ ((total ^ v[1]) + (k[0 ^ e] ^ z)))) & 0xFFFFFFFF
        total = (total - XXTEA_DELTA) & 0xFFFFFFFF
    return struct.pack(f'<{n}I', *v)


def patch_bundle(bundle_path, script_name, patch_fn):
    """Patch a specific script in a bundle. Returns True if patched."""
    env = UnityPy.load(bundle_path)
    patched = False

    for obj in env.objects:
        if obj.type.name != "MonoBehaviour":
            continue
        try:
            data = obj.read()
        except:
            continue

        name = getattr(data, 'm_Name', '') or ''
        encode = getattr(data, 'encode', False)
        raw = getattr(data, 'data', b'')
        if isinstance(raw, list):
            raw = bytes(raw)

        if name != script_name or not raw:
            continue

        # Decrypt
        if encode:
            decrypted = xxtea_decrypt(raw, XXTEA_KEY.encode('utf-8'))
        else:
            decrypted = raw

        try:
            text = decrypted.decode('utf-8')
        except:
            text = decrypted.decode('utf-8', errors='replace')

        # Apply patch
        new_text = patch_fn(text)
        if new_text is None or new_text == text:
            continue

        # Re-encode
        new_bytes = new_text.encode('utf-8')
        # Pad to multiple of 4 for XXTEA
        pad = (4 - len(new_bytes) % 4) % 4
        new_bytes_padded = new_bytes + b'\x00' * pad

        if encode:
            encrypted = xxtea_encrypt(new_bytes_padded, XXTEA_KEY.encode('utf-8'))
        else:
            encrypted = new_bytes_padded

        data.data = list(encrypted)
        data.save()
        patched = True
        print(f"  [PATCH] {name}: {len(text)} -> {len(new_text)} bytes")

    if patched:
        with open(bundle_path, 'wb') as f:
            f.write(env.file.save())
        print(f"  [SAVED] {bundle_path}")

    return patched


def patch_constants(text):
    """Patch Constants.lua to redirect to private server."""
    # Replace G_PACKAGE_TYPE
    text = text.replace('G_PACKAGE_TYPE = 4', 'G_PACKAGE_TYPE = 1')

    # Replace server URL
    text = text.replace(
        '    G_SERVER_URL = nil\n    -- G_SERVER_URL = "ws://10.100.5.56:9012"',
        '    G_SERVER_URL = "ws://127.0.0.1:9012"'
    )
    # Also handle the other nil assignments
    text = text.replace(
        'elseif bee.isDmod then\n    G_SERVER_URL = nil\nelseif bee.isPre then\n    G_SERVER_URL = nil\nelseif bee.isRelease then\n    G_SERVER_URL = nil',
        'elseif bee.isDmod then\n    G_SERVER_URL = "ws://127.0.0.1:9012"\nelseif bee.isPre then\n    G_SERVER_URL = "ws://127.0.0.1:9012"\nelseif bee.isRelease then\n    G_SERVER_URL = "ws://127.0.0.1:9012"'
    )
    # Fallback: just set it after the block
    if 'G_SERVER_URL = "ws://127.0.0.1:9012"' not in text:
        text = text.replace('G_SERVER_URL = nil', 'G_SERVER_URL = "ws://127.0.0.1:9012"')

    # Replace HTTP URLs
    text = text.replace(
        'G_HTTP_URL = "https://dev-login.poker-fate.com/"',
        'G_HTTP_URL = "http://127.0.0.1:8888/"'
    )
    # For dmod/pre/release, replace all HTTP URLs
    for old in [
        'G_HTTP_URL = "http://10.100.0.197/"',
        'G_HTTP_URL = "https://pre-login.poker-fate.com/"',
    ]:
        text = text.replace(old, 'G_HTTP_URL = "http://127.0.0.1:8888/"')

    # Replace release HTTP URLs (multi-line)
    if 'G_HTTP_URL = "http://127.0.0.1:8888/"' not in text:
        text = text.replace('G_HTTP_URL = "https://ga-foreign.poker-fate.com/"', 'G_HTTP_URL = "http://127.0.0.1:8888/"')

    # Null out backup URLs
    for var in ['G_HTTP_URL_2', 'G_HTTP_URL_3', 'G_HTTP_URL_4',
                'G_REMOTE_RES_HOST_2', 'G_REMOTE_RES_HOST_3',
                'G_RES_BASE_HOST_2', 'G_RES_BASE_HOST_3']:
        # Replace any assignment of these vars to nil
        import re
        text = re.sub(rf'{var}\s*=\s*"[^"]*"', f'{var} = nil', text)

    # Block hot-update: set remote res host to empty (prevents CDN download)
    text = text.replace(
        'G_REMOTE_RES_HOST = "https://dev-cdn.poker-fate.com/client/remote_res/dev/"',
        'G_REMOTE_RES_HOST = "" -- [PS] blocked hot-update'
    )
    text = text.replace(
        'G_REMOTE_RES_HOST = "https://dev-cdn.poker-fate.com/client/remote_res/dmod/"',
        'G_REMOTE_RES_HOST = "" -- [PS] blocked hot-update'
    )
    text = text.replace(
        'G_REMOTE_RES_HOST = "https://dev-cdn.poker-fate.com/client/remote_res/pre/"',
        'G_REMOTE_RES_HOST = "" -- [PS] blocked hot-update'
    )
    # Release remote hosts
    for old_host in [
        'https://aws.poker-fate.com/res/',
        'https://cdn.poker-fate.com/client/remote_res/release/',
        'https://bh-cn.oss-cn-shanghai.aliyuncs.com/res/',
    ]:
        text = text.replace(f'G_REMOTE_RES_HOST = "{old_host}"', 'G_REMOTE_RES_HOST = "" -- [PS] blocked')

    # Also block G_RES_BASE_HOST
    for old_host in [
        'https://dev-cdn.poker-fate.com',
        'https://djc1p2apfo64w.cloudfront.net',
        'https://cdn.poker-fate.com',
        'https://bh-cn.oss-cn-shanghai.aliyuncs.com',
    ]:
        text = text.replace(f'G_RES_BASE_HOST = "{old_host}"', 'G_RES_BASE_HOST = "" -- [PS] blocked')

    return text


def patch_sdk_helper(text):
    """Stub out SDK calls that would crash without real SDK."""
    # Add stubs at the end before 'return P'
    stubs = '''
-- [PS PATCH] Stub SDK calls
SdkHelper.adjustLaunched = function() end
SdkHelper.sentAdjustEvent = function() end
SdkHelper.sendFbEvent = function() end
SdkHelper.sendFirebaseEvent = function() end
SdkHelper.jumpGoogleReview = function() end
SdkHelper.startAppReview = function() end
-- [PS PATCH END]
'''
    if '-- [PS PATCH] Stub SDK calls' in text:
        return text  # Already patched
    text = text.replace('return P', stubs + '\nreturn P')
    return text


def patch_yidun_helper(text):
    """Replace YiDunHelper with stub."""
    # Full replacement
    return '''-- [PS PATCH] YiDunHelper stub
local P = {
    token = "",
    sdkVersion = "1.4.10",
    sdkVersion2 = "1.5.3",
    getedToken = false,
    reportData = {}
}
YiDunHelper = P

function P:init() end
function P:setToken(t) self.token = t self.getedToken = true end
function P:setRoleInfo() end
function P:setReportData(ch, ty, ac)
    self.reportData = {
        account = ac or "unknown",
        ip = "unknown",
        os = bee.pfsys,
        token = "",
        sceneData = {
            registerOrLogType = tostring(ch) or "iphone",
            operationType = ty or "register",
            appChannel = "1"
        }
    }
    bee.emit("evt_yidunTokenCallback")
end
function P:getReportData() return self.reportData end
-- [PS PATCH END]
'''


def patch_gameassembly():
    """Binary-patch GameAssembly.dll to bypass Steam init."""
    print("\n" + "=" * 60)
    print("GameAssembly.dll Binary Patcher")
    print("=" * 60)

    ga_path = Path(r"d:\GamePS\Poker Fate\Poker Fate\GameAssembly.dll")
    if not ga_path.exists():
        print("  [SKIP] GameAssembly.dll not found")
        return 0

    # Use the new version's GameAssembly.dll if available
    new_ga = Path(r"d:\GamePS\Poker Fate\new_ver_this_one\GameAssembly.dll")
    if new_ga.exists():
        ga_bak = ga_path.with_suffix('.dll.bak')
        shutil.copy2(new_ga, ga_path)
        print(f"  [COPY] New version GameAssembly.dll -> game dir")

    ga_bak = ga_path.with_suffix('.dll.bak')
    if not ga_bak.exists():
        shutil.copy2(ga_path, ga_bak)
        print(f"  [BACKUP] GameAssembly.dll.bak")
    else:
        shutil.copy2(ga_bak, ga_path)
        print(f"  [RESTORE] From backup (clean slate)")

    PATCHES = [
        # SteamHelper.init() @ file offset 0x54F510
        # Returns bool - patch to: mov al, 1; ret
        (0x54F510, bytes([0x48, 0x89, 0x5C, 0x24, 0x08, 0x57, 0x48, 0x83]),
         bytes([0xB0, 0x01, 0xC3, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamHelper.init() -> return true"),

        # SteamManager.Awake() @ file offset 0x54FD20
        # void function - patch to: ret
        (0x54FD20, bytes([0x48, 0x89, 0x5C, 0x24, 0x10, 0x48, 0x89, 0x4C]),
         bytes([0xC3, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamManager.Awake() -> ret"),

        # SteamManager.get_Initialized() @ file offset 0x550330
        # Returns bool - patch to: mov al, 1; ret
        (0x550330, bytes([0x40, 0x53, 0x48, 0x83, 0xEC, 0x20, 0x80, 0x3D]),
         bytes([0xB0, 0x01, 0xC3, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamManager.get_Initialized() -> return true"),

        # SteamManager.Update() @ file offset 0x550320
        # void function - patch to: ret
        (0x550320, bytes([0x80, 0x79, 0x20, 0x00, 0x74, 0x07, 0x33, 0xC9]),
         bytes([0xC3, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamManager.Update() -> ret"),
    ]

    applied = 0
    with open(ga_path, 'r+b') as f:
        for offset, orig, patch, desc in PATCHES:
            f.seek(offset)
            actual = f.read(len(orig))
            if actual == orig:
                f.seek(offset)
                f.write(patch)
                print(f"  [PATCH] 0x{offset:08X}: {desc}")
                applied += 1
            elif actual == patch:
                print(f"  [ALREADY] 0x{offset:08X}: {desc}")
                applied += 1
            else:
                print(f"  [MISMATCH] 0x{offset:08X}: expected {orig.hex()}, got {actual.hex()}")

    print(f"  GameAssembly.dll: {applied}/{len(PATCHES)} patches applied")
    return applied


def main():
    print("=" * 60)
    print("Poker Fate - New Version Binary Patcher")
    print("=" * 60)

    new_ver = Path(r"d:\GamePS\Poker Fate\new_ver_this_one\LocalFiles\remote_res")
    game_remote = Path(r"d:\GamePS\Poker Fate\Poker Fate\Poker Fate_Data\StreamingAssets\aa\StandaloneWindows64")

    # Step 1: Copy new version's LocalFiles to game's Addressables dir
    print("\n--- Step 1: Copy new version remote_res ---")
    # Unity Addressables stores downloaded bundles in the player's persistent data path,
    # not in StreamingAssets. Let's find the correct path.
    # On Windows: C:/Users/<user>/AppData/LocalLow/<company>/<game>/
    persistent_path = Path(r"C:\Users\45582\AppData\LocalLow\Poker Fate\Poker Fate")

    # The game stores Addressables catalog and downloaded bundles in persistent data path
    # We need to copy the remote_res folder there
    target_dir = persistent_path / "remote_res"

    if new_ver.exists():
        if target_dir.exists():
            shutil.rmtree(target_dir)
            print(f"  [CLEAN] Removed old {target_dir}")
        shutil.copytree(new_ver, target_dir)
        print(f"  [COPY] {new_ver} -> {target_dir}")
    else:
        print(f"  [ERROR] {new_ver} not found!")
        return

    # Step 2: Patch hot-update bundles
    print("\n--- Step 2: Patch hot-update bundles ---")

    patches = [
        # (bundle_filename, script_name, patch_function)
        ("gameres_assets_src/sdk_6ae6c042e06241b2263ff767fa786cd6.bundle", "SdkHelper", patch_sdk_helper),
        ("gameres_assets_src/sdk_6ae6c042e06241b2263ff767fa786cd6.bundle", "YiDunHelper", patch_yidun_helper),
        ("gameres_assets_src/app_91421dd8f87b4ee559c3e3d77c2271f5.bundle", "Constants", patch_constants),
    ]

    # Also patch the base bundle's init.lua (for YiDunHelper require)
    total = 0
    processed_bundles = set()
    for bundle_name, script_name, patch_fn in patches:
        bundle_path = target_dir / bundle_name
        if not bundle_path.exists():
            print(f"  [SKIP] {bundle_name} not found")
            continue
        # Backup
        bundle_bak = bundle_path.with_suffix('.bundle.bak')
        if not bundle_bak.exists():
            shutil.copy2(bundle_path, bundle_bak)
        elif bundle_path not in processed_bundles:
            # Restore from backup for clean patching
            shutil.copy2(bundle_bak, bundle_path)

        if patch_bundle(str(bundle_path), script_name, patch_fn):
            total += 1
        processed_bundles.add(bundle_path)

    # Step 3: Also patch the base bundle's init.lua to replace YiDunHelper require
    print("\n--- Step 3: Patch base bundle init.lua ---")
    base_bundle = game_remote / "gameres_assets_src_4facb2d11e7f1830a28ae0735170da85.bundle"
    if base_bundle.exists():
        base_bak = base_bundle.with_suffix('.bundle.bak')
        if not base_bak.exists():
            shutil.copy2(base_bundle, base_bak)
        else:
            shutil.copy2(base_bak, base_bundle)

        def patch_init(text):
            text = text.replace(
                'require "sdk.YiDunHelper"',
                '-- YiDunHelper stub (PS)\ndo local P = {token="",sdkVersion="1.4.10",sdkVersion2="1.5.3",getedToken=false,reportData={}} YiDunHelper = P function P:init() end function P:setToken(t) self.token=t self.getedToken=true end function P:setRoleInfo() end function P:setReportData(ch,ty,ac) self.reportData={account=ac or "unknown",ip="unknown",os=bee.pfsys,token="",sceneData={registerOrLogType=tostring(ch) or "iphone",operationType=ty or "register",appChannel="1"}} bee.emit("evt_yidunTokenCallback") end function P:getReportData() return self.reportData end end\n'
            )
            return text

        if patch_bundle(str(base_bundle), "init", patch_init):
            total += 1

    # Step 4: Patch main.lua in base bundle
    print("\n--- Step 4: Patch base bundle main.lua ---")
    if base_bundle.exists():
        def patch_main(text):
            # Add URL overrides after require "app.Constants"
            override = '''
-- [PS PATCH] Override server URLs
G_PACKAGE_TYPE = 1
G_SERVER_URL = "ws://127.0.0.1:9012"
G_HTTP_URL = "http://127.0.0.1:8888/"
G_HTTP_URL_2 = nil
G_HTTP_URL_3 = nil
G_HTTP_URL_4 = nil
G_REMOTE_RES_HOST = ""
G_RES_BASE_HOST = ""
G_SHARE_URL = ""
bee.isTest = true
bee.isDev = true
bee.isDmod = false
bee.isPre = false
bee.isRelease = false
bee.isInTest = false
-- [PS PATCH END]
'''
            if '-- [PS PATCH] Override server URLs' in text:
                return text  # Already patched
            text = text.replace(
                'require "app.Constants"',
                'require "app.Constants"\n' + override
            )
            # Stub SDK calls
            sdk_stubs = '''
-- [PS PATCH] Stub SDK calls
SdkHelper.adjustLaunched = function() end
SdkHelper.sentAdjustEvent = function() end
SdkHelper.sendFbEvent = function() end
SdkHelper.sendFirebaseEvent = function() end
SdkHelper.jumpGoogleReview = function() end
SdkHelper.startAppReview = function() end
-- [PS PATCH END]
'''
            if '-- [PS PATCH] Stub SDK calls' not in text:
                text = text.replace('SdkHelper.adjustLaunched()', sdk_stubs + '\nSdkHelper.adjustLaunched()')
            return text

        if patch_bundle(str(base_bundle), "main", patch_main):
            total += 1

    # Step 5: Patch GameAssembly.dll
    print("\n--- Step 5: Patch GameAssembly.dll ---")
    ga_patches = patch_gameassembly()
    total += ga_patches

    # Step 6: Copy NEP2.dll stub
    print("\n--- Step 6: NEP2.dll stub ---")
    nep2_src = Path(r"d:\GamePS\Poker Fate\PokerFatePatch\nep2_stub\NEP2.dll")
    nep2_dst = Path(r"d:\GamePS\Poker Fate\Poker Fate\NEP2.dll")
    if nep2_src.exists():
        if not nep2_dst.with_suffix('.dll.bak').exists() and nep2_dst.exists():
            shutil.copy2(nep2_dst, nep2_dst.with_suffix('.dll.bak'))
        shutil.copy2(nep2_src, nep2_dst)
        print(f"  [COPY] NEP2.dll stub")

    # Step 7: Copy new version steam_api64.dll (Goldberg emulator)
    print("\n--- Step 7: Steam emulator ---")
    steam_src = Path(r"d:\GamePS\Poker Fate\Poker Fate\Poker Fate_Data\Plugins\x86_64\steam_api64.dll.bak")
    # Keep the existing Goldberg emulator setup
    if not steam_src.exists():
        print("  [OK] Steam emulator already in place")

    print(f"\n{'='*60}")
    print(f"Total patches applied: {total}")
    print(f"{'='*60}")


if __name__ == "__main__":
    main()
