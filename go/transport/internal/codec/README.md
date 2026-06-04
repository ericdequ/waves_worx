# Transport Codec Helpers

`transport/internal/codec` contains tiny wire-format helpers used by BEV mobile
transport packages.

## Role

- `PackGeohash8` / `UnpackGeohash8` for fixed-width nearby-location hints.
- `XORChecksum` for cheap frame corruption detection.
- `IsGeohashChar` for transport-safe validation.

The package is intentionally `internal`: it is shared only by mobile transport
implementations and is not a general public API. Reusable geohash/location
helpers should go through `GO/pkg/placecore` or `GO/pkg/geoschema`.
