# XXTEA Encryption

The Lua scripts inside Poker Fate's Unity Addressables bundles are stored XXTEA-encrypted. The patch scripts in this package implement XXTEA in pure Python (no native dependencies).

## Parameters

| Parameter | Value |
|---|---|
| Key | `bee#happy&pkproject` |
| Delta | `0x9E3779B9` |

## Algorithm Notes

- Standard XXTEA (also known as Corrected Block TEA).
- Data is padded to a multiple of 4 bytes before encryption.
- Key is 16 bytes (the ASCII bytes of the key string above, zero-padded if shorter — but the key above is exactly 20 bytes, so only the first 16 are used).
- Number of rounds: `6 + 52 / n` where `n` is the number of 32-bit words.
- Endianness: little-endian (`<I` struct format).

## Reference Implementation

See `xxtea_encrypt` and `xxtea_decrypt` in `patch_new_ver.py` (lines 17-60) for the canonical Python implementation used by all patch scripts in this package.

## Verifying

To verify the key/delta against a fresh bundle, decrypt any Lua MonoBehaviour's `m_Script` bytes and check that the result starts with a valid Lua bytecode or source header (e.g. `local` or `\x1bLua`).
