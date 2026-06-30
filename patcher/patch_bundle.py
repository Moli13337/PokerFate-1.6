"""
Patch Constants.lua inside the app bundle to redirect all server URLs to localhost.
Also disables Addressables catalog update on start.
"""
import os
import struct
import sys
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


# ==================== Config ====================
LUA_KEY = "bee#happy&pkproject"

# Bundles containing Constants.lua (remote_res is the primary one)
BUNDLE_PATHS = [
    Path(r"d:\GamePS\Poker Fate\new_ver_this_one\LocalFiles\remote_res\gameres_assets_src\app_91421dd8f87b4ee559c3e3d77c2271f5.bundle"),
]

SETTINGS_JSON = Path(r"d:\GamePS\Poker Fate\new_ver_this_one\Poker Fate_Data\StreamingAssets\aa\settings.json")

# URL replacements: old -> new
URL_REPLACEMENTS = [
    # HTTP API server
    ('G_HTTP_URL = "https://ga-foreign.poker-fate.com/"',
     'G_HTTP_URL = "http://127.0.0.1:8888/"'),
    ('G_HTTP_URL_2 = "https://awsb-entry.poker-fate.com/"',
     'G_HTTP_URL_2 = "http://127.0.0.1:8888/"'),
    ('G_HTTP_URL_3 = "http://8.163.49.33:8888/"',
     'G_HTTP_URL_3 = "http://127.0.0.1:8888/"'),
    ('G_HTTP_URL_4 = "http://121.196.174.32:8888/"',
     'G_HTTP_URL_4 = "http://127.0.0.1:8888/"'),
    # Remote resource host (for hot update version check)
    ('G_REMOTE_RES_HOST = "https://aws.poker-fate.com/res/"',
     'G_REMOTE_RES_HOST = "http://127.0.0.1:8888/client/remote_res/release/"'),
    ('G_REMOTE_RES_HOST_2 = "https://cdn.poker-fate.com/client/remote_res/release/"',
     'G_REMOTE_RES_HOST_2 = "http://127.0.0.1:8888/client/remote_res/release/"'),
    ('G_REMOTE_RES_HOST_3 = "https://bh-cn.oss-cn-shanghai.aliyuncs.com/res/"',
     'G_REMOTE_RES_HOST_3 = "http://127.0.0.1:8888/client/remote_res/release/"'),
    # Resource base host (for bundle downloads)
    ('G_RES_BASE_HOST = "https://djc1p2apfo64w.cloudfront.net"',
     'G_RES_BASE_HOST = "http://127.0.0.1:8888"'),
    ('G_RES_BASE_HOST_2 = "https://cdn.poker-fate.com"',
     'G_RES_BASE_HOST_2 = "http://127.0.0.1:8888"'),
    ('G_RES_BASE_HOST_3 = "https://bh-cn.oss-cn-shanghai.aliyuncs.com"',
     'G_RES_BASE_HOST_3 = "http://127.0.0.1:8888"'),
]


def patch_bundle(bundle_path):
    print(f"\n{'='*60}")
    print(f"Patching: {bundle_path.name}")
    print(f"{'='*60}")

    if not bundle_path.exists():
        print(f"  [SKIP] Bundle not found")
        return False

    # Backup
    backup = bundle_path.with_suffix('.bundle.bak')
    if not backup.exists():
        shutil.copy2(bundle_path, backup)
        print(f"  [BACKUP] Saved to {backup.name}")

    env = UnityPy.load(str(bundle_path))

    # Build path lookup from container: path_id -> container_path
    path_map = {}
    for cpath, pptr in env.container.items():
        try:
            path_map[pptr.path_id] = cpath
        except Exception:
            pass

    patched = False
    for obj in env.objects:
        if obj.type.name != "MonoBehaviour":
            continue
        cpath = path_map.get(obj.path_id, "")
        if 'Constants' not in cpath:
            continue

        tree = obj.read_typetree()
        if not isinstance(tree, dict):
            continue

        name = tree.get('m_Name', '')
        raw_data = ensure_bytes(tree.get('data', b''))
        encode = tree.get('encode', False)
        decode_key = tree.get('LuaDecodeKey', '') or ''

        print(f"  Found: {cpath}")
        print(f"    m_Name={name!r}, encode={encode}, key={decode_key!r}, data_len={len(raw_data)}")

        # Decrypt
        if encode:
            key = decode_key if decode_key else LUA_KEY
            decrypted = xxtea_decrypt(raw_data, key)
            if not decrypted:
                for k in ["bee#happy&pkproto", "poker_fate_key", "pkproject"]:
                    decrypted = xxtea_decrypt(raw_data, k)
                    if decrypted:
                        key = k
                        break
            if not decrypted:
                print(f"  [FAIL] Could not decrypt with any key")
                return False
            print(f"  [DECRYPT] OK with key={key!r}, len={len(decrypted)}")
        else:
            decrypted = raw_data
            key = None

        text = decrypted.decode('utf-8', errors='replace')

        # Apply URL replacements
        changes = 0
        for old, new in URL_REPLACEMENTS:
            if old in text:
                text = text.replace(old, new)
                changes += 1
                print(f"  [PATCH] {old[:60]}... -> localhost")

        if changes == 0:
            print(f"  [SKIP] No URL patterns matched (already patched?)")
            return True

        # Re-encrypt
        new_data = text.encode('utf-8')
        if encode and key:
            encrypted = xxtea_encrypt(new_data, key)
            tree['data'] = list(encrypted)
        else:
            tree['data'] = list(new_data)

        obj.save_typetree(tree)
        patched = True
        print(f"  [ENCRYPT] Re-encrypted with key={key!r}, new_len={len(new_data)}")

    if not patched:
        print(f"  [WARN] Constants.lua not found in this bundle")
        return False

    # Save bundle
    with open(bundle_path, 'wb') as f:
        f.write(env.file.save())

    print(f"  [SAVED] Bundle written to {bundle_path}")
    return True


def patch_settings_json():
    print(f"\n{'='*60}")
    print(f"Patching: settings.json (disable catalog update)")
    print(f"{'='*60}")

    if not SETTINGS_JSON.exists():
        print(f"  [SKIP] settings.json not found")
        return False

    # Backup
    backup = SETTINGS_JSON.with_suffix('.json.bak')
    if not backup.exists():
        shutil.copy2(SETTINGS_JSON, backup)
        print(f"  [BACKUP] Saved to {backup.name}")

    text = SETTINGS_JSON.read_text(encoding='utf-8')
    if '"m_DisableCatalogUpdateOnStart":false' in text:
        text = text.replace('"m_DisableCatalogUpdateOnStart":false', '"m_DisableCatalogUpdateOnStart":true')
        SETTINGS_JSON.write_text(text, encoding='utf-8')
        print(f"  [PATCH] m_DisableCatalogUpdateOnStart: false -> true")
    elif '"m_DisableCatalogUpdateOnStart":true' in text:
        print(f"  [SKIP] Already disabled")
    else:
        print(f"  [WARN] Pattern not found")
        return False

    return True


def main():
    print("Poker Fate - Bundle Patcher (redirect to localhost)")

    ok1 = patch_settings_json()
    ok2 = False
    for bp in BUNDLE_PATHS:
        if patch_bundle(bp):
            ok2 = True

    print(f"\n{'='*60}")
    print("SUMMARY")
    print(f"{'='*60}")
    print(f"  settings.json patched: {ok1}")
    print(f"  Constants.lua patched: {ok2}")
    if ok1 and ok2:
        print("\n  All patches applied. The game should now connect to:")
        print("    HTTP API:  http://127.0.0.1:8888/")
        print("    WS Server: ws://127.0.0.1:9012")
        print("    Res Host:  http://127.0.0.1:8888/client/remote_res/release/")


if __name__ == "__main__":
    main()
