# transport/vlc — Visible Light chunked payload

Screen-to-camera "QR-code-pulse" chunker for the **fun-mode** stack-phones-
together transport. **Research-grade**, not a primary data path. ~1 kbps
after error correction.

## Lift-and-shift dependencies

`transport/` only. Pure stdlib (`hash/crc32`).

## Chunk layout (66 bytes)

```
[0..1]   chunk index (uint16 BE)
[2..3]   total chunks (uint16 BE)
[4..63]  chunk payload (60 bytes, zero-padded if last)
[64..65] CRC16 of [0..63]
```

## Caveat on trailing zero bytes

Decode trims trailing zero padding. If your payload legitimately ends in
`0x00`, prefix it with a length yourself.

## Fallback chain

| Platform | Chain                          | Why                                  |
|----------|--------------------------------|--------------------------------------|
| any      | `[vlc, sonic, ble]`            | VLC rare; sonic next for near-field  |
| web      | `[sonic, ble]`                 | No native camera loop on web         |

## Apply ops

- `transport.encode_vlc` / `transport.decode_vlc`
