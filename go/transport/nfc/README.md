# transport/nfc — NDEF handshake record

NFC NDEF external-type record encoder/decoder for one-shot key exchange via
phone-tap. Record type: `application/vnd.bev.handshake.v1`.

## ⚠️ Platform reality

**iOS does not support NFC peer-to-peer**. Apple removed it in iOS 13.
Apps can read NDEF tags + write tags, but **two iPhones cannot exchange
NFC packets**. This package's Go encoder is platform-agnostic, but the
radio integration only works Android↔Android.

The fallback chain reflects this honestly — iOS routes around NFC entirely.

## Lift-and-shift dependencies

`transport/` only. Pure stdlib otherwise (`encoding/binary`).

## Payload (108 bytes)

```
[0..3]    protocol version + flags (uint32 BE)
[4..35]   peer Ed25519 public key (32 bytes)
[36..43]  minted-at unix seconds (uint64 BE)
[44..107] Ed25519 signature of (pubkey || mintedAt)
```

## Fallback chain

| Platform | Chain                          | Why                                  |
|----------|--------------------------------|--------------------------------------|
| android  | `[nfc, uwb, sonic]`            | NFC works; UWB on Android 14+ second |
| ios      | `[uwb, sonic, ble]`            | NFC peer dead; UWB tap on iPhones    |
| web      | `[sonic]`                      | No NFC on web                        |

## Apply ops

- `transport.encode_nfc_handshake` — pack `Payload` (with caller's signature)
- `transport.decode_nfc_handshake` — unpack + return `SigningPayload` for Ed25519 verify on native side

## Signing flow (caller responsibility)

Go's middle-end never sees private keys.

```ts
// On Android tap:
const signing = await BevGoCore.coreApply({
    operation: 'transport.decode_nfc_handshake',
    payload: JSON.stringify({ payload: ndefBytes }),
});
const { payload, signingPayload } = JSON.parse(signing.result);
// Native: verify ed25519.Verify(payload.publicKey, signingPayload, payload.signature)
```
