# transport/uwb — Ultra-Wideband ranging interpretation

No frame encoding — UWB radios broadcast timestamps, not user payloads.
This package classifies native UWB observations (distance + angles) into
trust verdicts and feeds the aggregator.

## ⚠️ Cross-platform reality

- **iPhone 11+** (U1/U2 chip) via `NearbyInteraction` framework, iOS 14+. Works.
- **Android** UWB API 31+ (Android 12+), but fragmented across vendors.
- **iOS↔Android UWB** is possible via the FiRa Consortium spec but still rocky as of 2026.

## Lift-and-shift dependencies

`transport/` only.

## MeetupVerdict tiers

```
DistanceM ≤  1m  → handshake  (deliberate tap-to-meet)
DistanceM ≤  3m  → nearby     (conversational distance)
DistanceM ≤ 10m  → sameRoom   (plausibly same room)
DistanceM >  10m → distant    (low-trust)
```

Low confidence (<0.6) demotes the verdict one tier — conservative bias.

## Fallback chain

| Platform | Chain                          | Why                          |
|----------|--------------------------------|------------------------------|
| ios      | `[uwb, sonic, ble]`            | UWB on iPhone 11+            |
| android  | `[uwb, sonic, ble]`            | API 31+ only                 |
| any      | `[sonic, ble]`                 | No UWB capability → sonic    |

## Apply ops

- `transport.record_uwb` — ingest a native observation; returns verdict + new trust score
