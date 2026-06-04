# transport/wifi — Wi-Fi P2P service-info envelope

Cross-platform envelope for Wi-Fi peer discovery. Same JSON shape across
Android Wi-Fi Aware (NAN) and iOS Multipeer Connectivity (MPC).

## ⚠️ Cross-OS gap

**Wi-Fi Aware and Multipeer Connectivity do not interoperate.** Android can't
discover an iPhone over Wi-Fi P2P, and vice versa. Mixed-platform groups
must fall back to BLE for envelope + RTDB for bulk.

## Lift-and-shift dependencies

`transport/` only.

## Envelope

```json
{
  "v": 1,
  "k": "identity",
  "pid": "peer-abc",
  "g9": "9q8yyk8yp",
  "t": 1715000000,
  "sig": "base64..."
}
```

## Fallback chain

| Platform | Chain                | Why                                  |
|----------|----------------------|--------------------------------------|
| any      | `[wifi_p2p, ble]`    | BLE carries the envelope; bulk → RTDB |
| web      | `[ble]`              | No Wi-Fi P2P on web                  |

## Apply ops

- `transport.encode_wifi` / `transport.decode_wifi`
