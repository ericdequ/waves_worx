# transport/ble — BLE manufacturer-data frames

Frame encoder/decoder for BEV's BLE advertisement payload. Pairs with the
native BLE plugin (`vendor/capacitor-go-core/ios + android`) which owns
the radio; this Go package owns the bytes.

## Lift-and-shift dependencies

Only `transport/` (sibling, for `VectorID` + fallback registration) and
`transport/internal/codec/` (XOR checksum + geohash alphabet). Pure stdlib
otherwise. Drop the three directories into another project + adjust import
paths and it works.

## Frame layout (21 bytes)

```
[0]      version (1)
[1]      kind: 1=vibe, 2=identity, 3=gossip-tease
[2..8]   geohash7 (ASCII; "0000000" = no location)
[9..10]  vibe score (uint16 BE)
[11..14] rotating ephemeral peer ID (4 bytes)
[15..16] HMAC-SHA256 truncation (2 bytes; anti-replay)
[17..19] reserved
[20]     XOR checksum of [0..19]
```

## Usage

```go
import "github.com/ericdequ/BEV/GO/mobile/bevcore/transport/ble"

frame, err := ble.EncodeFrame(ble.Payload{
    Kind: ble.KindVibe,
    Geohash7: "9q8yyk8",
    VibeScore: 42,
    EphemeralID: []byte{0xDE,0xAD,0xBE,0xEF},
}, hmacKey)

// On scanner side:
p, err := ble.DecodeFrame(frame)
ok := ble.VerifyHMAC(frame, hmacKey)
```

## Fallback chain

Registered at init():

| Platform | Chain                              |
|----------|------------------------------------|
| any      | `[ble, sonic, wifi_p2p]`           |

BLE is universal on modern phones; fallbacks are rarely needed. Sonic is
listed second because it works even when BLE is jammed (e.g., dense
advertising contention in a crowded venue). Wi-Fi P2P third for the
higher-bandwidth case where BLE's 21-byte frame is too small.

## Apply ops surfaced through bevcore dispatch

- `transport.encode_ble` — pack a `Payload` into bytes
- `transport.decode_ble` — unpack bytes back into a `Payload` + HMAC check

## Native plugin contract

The Capacitor plugin must call back into Go via the bevcore Apply ops:

```ts
// On scanner: a frame arrived
const result = await BevGoCore.coreApply({
    operation: 'transport.decode_ble',
    payload: JSON.stringify({ frame: bytes, hmacKey: lobbyKey }),
});
const { payload, hmacValid } = JSON.parse(result.result);

if (hmacValid) {
    await BevGoCore.coreApply({
        operation: 'transport.record_observation',
        payload: JSON.stringify({
            vector: 'ble',
            peerId: toHex(payload.ephemeralId),
            rssi: scanResult.rssi,
            observedAtMs: Date.now(),
        }),
    });
}
```
