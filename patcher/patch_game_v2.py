"""
Poker Fate - Binary Patch Script v2
Patches the 'main' and 'init' Lua scripts in the src bundle.
Strategy: Inject URL overrides and SDK stubs directly into main.lua,
since Constants.lua is loaded via hot-update and not in the base bundles.
"""

import os
import struct
import shutil
from pathlib import Path
import UnityPy

# ==================== XXTEA ====================
DELTA = 0x9E3779B9

def _mx(sum_val, y, z, p, e, k):
    return ((z >> 5 ^ y << 2) + (y >> 3 ^ z << 4)) ^ ((sum_val ^ y) + (k[(p & 3) ^ e] ^ z))

def _fix_key(key_bytes):
    if len(key_bytes) < 16:
        key_bytes = key_bytes + b'\x00' * (16 - len(key_bytes))
    elif len(key_bytes) > 16:
        key_bytes = key_bytes[:16]
    return key_bytes

def _to_uint32_array(data, include_length):
    length = len(data)
    n = length // 4
    if length % 4 != 0:
        data = data + b'\x00' * (4 - length % 4)
        n += 1
    result = list(struct.unpack_from(f'<{n}I', data))
    if include_length:
        result.append(length)
    return result

def _to_byte_array(data, include_length):
    result = struct.pack(f'<{len(data)}I', *data)
    if include_length and len(data) > 0:
        actual_length = data[-1]
        if actual_length < len(result):
            result = result[:actual_length]
    return result

def _xxtea_core_encrypt(v, k):
    n = len(v)
    if n < 2:
        return
    rounds = 6 + 52 // n
    sum_val = 0
    z = v[n - 1]
    for _ in range(rounds):
        sum_val = (sum_val + DELTA) & 0xFFFFFFFF
        e = (sum_val >> 2) & 3
        for p in range(n - 1):
            y = v[p + 1]
            mx_val = _mx(sum_val, y, z, p, e, k)
            v[p] = (v[p] + mx_val) & 0xFFFFFFFF
            z = v[p]
        y = v[0]
        mx_val = _mx(sum_val, y, z, n - 1, e, k)
        v[n - 1] = (v[n - 1] + mx_val) & 0xFFFFFFFF
        z = v[n - 1]

def _xxtea_core_decrypt(v, k):
    n = len(v)
    if n < 2:
        return
    rounds = 6 + 52 // n
    sum_val = (rounds * DELTA) & 0xFFFFFFFF
    y = v[0]
    for _ in range(rounds):
        e = (sum_val >> 2) & 3
        for p in range(n - 1, 0, -1):
            z = v[p - 1]
            mx_val = _mx(sum_val, y, z, p, e, k)
            v[p] = (v[p] - mx_val) & 0xFFFFFFFF
            y = v[p]
        z = v[n - 1]
        mx_val = _mx(sum_val, y, z, 0, e, k)
        v[0] = (v[0] - mx_val) & 0xFFFFFFFF
        y = v[0]
        sum_val = (sum_val - DELTA) & 0xFFFFFFFF

def xxtea_encrypt(data_bytes, key_str):
    key_bytes = _fix_key(key_str.encode('utf-8'))
    v = _to_uint32_array(data_bytes, True)
    k = _to_uint32_array(key_bytes, False)
    _xxtea_core_encrypt(v, k)
    return _to_byte_array(v, False)

def xxtea_decrypt(data_bytes, key_str):
    if not data_bytes or len(data_bytes) < 8:
        return None
    key_bytes = _fix_key(key_str.encode('utf-8'))
    v = _to_uint32_array(data_bytes, False)
    k = _to_uint32_array(key_bytes, False)
    _xxtea_core_decrypt(v, k)
    return _to_byte_array(v, True)

LUA_KEY = "bee#happy&pkproject"

def ensure_bytes(data):
    if isinstance(data, bytes):
        return data
    if isinstance(data, memoryview):
        return bytes(data)
    if isinstance(data, list):
        return bytes(data)
    if hasattr(data, 'tobytes'):
        return data.tobytes()
    return bytes(data)

# ==================== Patched Scripts ====================

# Patch main.lua to override Constants AFTER it's loaded
PATCHED_MAIN = r'''--print("===== load main.lua =====", jit)
if jit and jit.off then
    jit.off()
    print("off jit ")
end

CS.SdkHelper.InitGame()
require "engine.init"
require "app.Constants"

-- [PS PATCH] Override server URLs
G_PACKAGE_TYPE = 1
G_SERVER_URL = "ws://127.0.0.1:9012"
G_HTTP_URL = "http://127.0.0.1:8888/"
G_HTTP_URL_2 = nil
G_HTTP_URL_3 = nil
G_HTTP_URL_4 = nil
G_REMOTE_RES_HOST = "http://127.0.0.1:8888/"
G_RES_BASE_HOST = "http://127.0.0.1:8888/"
G_SHARE_URL = ""
bee.isTest = true
bee.isDev = true
bee.isDmod = false
bee.isPre = false
bee.isRelease = false
bee.isInTest = false
-- [PS PATCH END]

require "manager.LogTool"
require "ui.init"
require "net.init"
print("main.lua isOpen",CS.AppLoader.isOpen,"isReload ",CS.AppLoader.isReload)
    require "app.init"

    require "appload.AppLoadRes"

-- [PS PATCH] Stub SDK calls
SdkHelper.adjustLaunched = function() end
SdkHelper.sentAdjustEvent = function() end
SdkHelper.sendFbEvent = function() end
SdkHelper.sendFirebaseEvent = function() end
SdkHelper.jumpGoogleReview = function() end
SdkHelper.startAppReview = function() end
-- [PS PATCH END]

SdkHelper.adjustLaunched()
LogTool:init()

if bee.isEditor then
	local _, LuaDebuggee = pcall(require, 'LuaDebuggee')
	if LuaDebuggee and LuaDebuggee.StartDebug then
		if LuaDebuggee.StartDebug('127.0.0.1', 9826) then
			print('LuaPerfect: Successfully connected to debugger!')
		else
			print('LuaPerfect: Failed to connect debugger!')
		end
	else
		print('LuaPerfect: Check documents at: https://luaperfect.net')
	end
end
'''

# Patch YiDunHelper: skip all DeviceFingerprint calls
YIDUN_PATCH = r'''local P = {
    token = "",
    sdkVersion = "1.4.10",
    sdkVersion2 = "1.5.3",
    getedToken = false,
    reportData = {},
}
YiDunHelper = P

function P:init()
end

function P:setToken(token)
    self.token = token
    self.getedToken = true
end

function P:setRoleInfo()
end

function P:setReportData(channel, type, account)
    self.reportData = {}
    self.reportData.account = account or "unknown"
    self.reportData.ip = "unknown"
    self.reportData.os = bee.pfsys
    self.reportData.token = ""
    self.reportData.sceneData = {
        registerOrLogType = tostring(channel) or "iphone",
        operationType = type or "register",
        appChannel = "1",
    }
    bee.emit("evt_yidunTokenCallback")
end

function P:getReportData()
    return self.reportData
end

return P
'''

# ==================== Bundle Patcher ====================

BUNDLE_DIR = Path(r"d:\GamePS\Poker Fate\Poker Fate\Poker Fate_Data\StreamingAssets\aa\StandaloneWindows64")
BACKUP_DIR = Path(r"d:\GamePS\Poker Fate\Poker Fate\Poker Fate_Data\StreamingAssets\aa\StandaloneWindows64_backup")
SRC_BUNDLE = BUNDLE_DIR / "gameres_assets_src_4facb2d11e7f1830a28ae0735170da85.bundle"

def patch_bundle():
    print("=" * 60)
    print("Poker Fate - Binary Lua Patcher v2")
    print("=" * 60)

    if not BACKUP_DIR.exists():
        print(f"\nCreating backup...")
        shutil.copytree(BUNDLE_DIR, BACKUP_DIR)
    else:
        print(f"\nRestoring from backup...")
        shutil.rmtree(BUNDLE_DIR)
        shutil.copytree(BACKUP_DIR, BUNDLE_DIR)

    patched = 0

    env = UnityPy.load(str(SRC_BUNDLE))
    modified = False

    for obj in env.objects:
        if obj.type.name != "MonoBehaviour":
            continue
        try:
            tree = obj.read_typetree()
        except:
            continue
        if not isinstance(tree, dict):
            continue

        name = tree.get('m_Name', '')
        encode = tree.get('encode', False)
        data = tree.get('data')

        if not encode or data is None:
            continue

        data = ensure_bytes(data)
        dec = xxtea_decrypt(data, LUA_KEY)
        if not dec:
            continue

        text = dec.decode('utf-8', errors='replace')

        new_text = None

        if name == 'main':
            new_text = PATCHED_MAIN
            print(f"  [PATCH] main.lua")

        elif name == 'YiDunHelper' or (name == 'init' and b'YiDunHelper' in dec):
            # If YiDunHelper is in the 'init' mega-script, replace the require with a stub
            if name == 'init' and 'require "sdk.YiDunHelper"' in text:
                text = text.replace(
                    'require "sdk.YiDunHelper"',
                    '-- YiDunHelper stub (PS)\ndo local P = {token="",sdkVersion="1.4.10",sdkVersion2="1.5.3",getedToken=false,reportData={}} YiDunHelper = P function P:init() end function P:setToken(t) self.token=t self.getedToken=true end function P:setRoleInfo() end function P:setReportData(ch,ty,ac) self.reportData={account=ac or "unknown",ip="unknown",os=bee.pfsys,token="",sceneData={registerOrLogType=tostring(ch) or "iphone",operationType=ty or "register",appChannel="1"}} bee.emit("evt_yidunTokenCallback") end function P:getReportData() return self.reportData end end\n'
                )
                new_text = text
                print(f"  [PATCH] init.lua - injected YiDunHelper stub")

        if new_text:
            new_bytes = new_text.encode('utf-8')
            new_encrypted = xxtea_encrypt(new_bytes, LUA_KEY)
            tree['data'] = list(new_encrypted)
            obj.save_typetree(tree)
            modified = True
            patched += 1
            print(f"  [OK] {name}: {len(data)} -> {len(new_encrypted)} bytes")

    if modified:
        with open(SRC_BUNDLE, 'wb') as f:
            f.write(env.file.save())
        print(f"  [SAVED] {SRC_BUNDLE.name}")

    ga_patches = patch_gameassembly()
    patched += ga_patches

    print(f"\nTotal patches: {patched}")
    return patched


def patch_gameassembly():
    """Binary-patch GameAssembly.dll to bypass Steam init and anti-cheat."""
    print("\n" + "=" * 60)
    print("GameAssembly.dll Binary Patcher")
    print("=" * 60)

    ga_path = Path(r"d:\GamePS\Poker Fate\Poker Fate\GameAssembly.dll")
    if not ga_path.exists():
        print("  [SKIP] GameAssembly.dll not found")
        return 0

    # Backup
    ga_bak = ga_path.with_suffix('.dll.bak')
    if not ga_bak.exists():
        shutil.copy2(ga_path, ga_bak)
        print(f"  [BACKUP] GameAssembly.dll -> GameAssembly.dll.bak")
    else:
        # Restore from backup first to ensure clean patching
        shutil.copy2(ga_bak, ga_path)
        print(f"  [RESTORE] From backup (clean slate)")

    # Define patches: (offset, original_bytes, patch_bytes, description)
    # All offsets are file offsets from il2cpp dump
    PATCHES = [
        # SteamHelper.init() @ file offset 0x54F510
        # Returns bool - patch to: mov al, 1; ret
        (0x54F510, bytes([0x48, 0x89, 0x5C, 0x24, 0x08, 0x57, 0x48, 0x83]),
         bytes([0xB0, 0x01, 0xC3, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamHelper.init() -> return true"),

        # SteamManager.Awake() @ file offset 0x54FD20
        # void function - patch to: ret (skip all Steam init)
        (0x54FD20, bytes([0x48, 0x89, 0x5C, 0x24, 0x10, 0x48, 0x89, 0x4C]),
         bytes([0xC3, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamManager.Awake() -> ret (skip Steam init)"),

        # SteamManager.get_Initialized() @ file offset 0x550330
        # Returns bool - patch to: mov al, 1; ret
        (0x550330, bytes([0x40, 0x53, 0x48, 0x83, 0xEC, 0x20, 0x80, 0x3D]),
         bytes([0xB0, 0x01, 0xC3, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamManager.get_Initialized() -> return true"),

        # SteamManager.Update() @ file offset 0x550320
        # void function - patch to: ret (skip SteamAPI.RunCallbacks)
        (0x550320, bytes([0x80, 0x79, 0x20, 0x00, 0x74, 0x07, 0x33, 0xC9]),
         bytes([0xC3, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90]),
         "SteamManager.Update() -> ret (skip RunCallbacks)"),
    ]

    with open(ga_path, 'r+b') as f:
        applied = 0
        for offset, orig, patch, desc in PATCHES:
            f.seek(offset)
            actual = f.read(len(orig))
            if actual == orig:
                f.seek(offset)
                f.write(patch)
                print(f"  [PATCH] 0x{offset:08X}: {desc}")
                applied += 1
            elif actual == patch:
                print(f"  [ALREADY PATCHED] 0x{offset:08X}: {desc}")
                applied += 1
            else:
                print(f"  [MISMATCH] 0x{offset:08X}: expected {orig.hex()}, got {actual.hex()}")
                print(f"    Description: {desc}")

    print(f"  GameAssembly.dll: {applied}/{len(PATCHES)} patches applied")
    return applied

if __name__ == "__main__":
    patch_bundle()
