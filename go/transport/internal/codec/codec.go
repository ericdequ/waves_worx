// Package codec holds the tiny wire-format helpers each transport
// subpackage uses. Internal because consumers outside transport/ shouldn't
// be poking at frame internals.
package codec

// GeohashAlphabet is the base32 alphabet BEV uses (excludes a/i/l/o per the
// project_id_system convention). Public so transport frame encoders can
// validate geohash chars uniformly.
const GeohashAlphabet = "0123456789bcdefghjkmnpqrstuvwxyz"

// XORChecksum returns the bitwise XOR of all bytes. Cheap integrity check
// suitable for the short fixed-length frames in this package.
func XORChecksum(b []byte) byte {
	var x byte
	for _, v := range b {
		x ^= v
	}
	return x
}

// IsGeohashChar reports whether b is a valid BEV-base32 geohash character.
func IsGeohashChar(b byte) bool {
	for i := 0; i < len(GeohashAlphabet); i++ {
		if GeohashAlphabet[i] == b {
			return true
		}
	}
	return false
}
