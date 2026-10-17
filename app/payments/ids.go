package payments

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"regexp"
	"time"
)

// NewPublicID returns a UUIDv7 (RFC 9562): a 48-bit Unix millisecond
// timestamp and 74 random bits, so ids sort by creation and never reveal
// how many orders exist.
//
// workaround(vukem): no UUID helper in vukem.
func NewPublicID(now time.Time) string {
	var b [16]byte

	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails

	var ms [8]byte

	binary.BigEndian.PutUint64(ms[:], uint64(now.UnixMilli()))
	copy(b[:6], ms[2:])

	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 9562 variant

	h := hex.EncodeToString(b[:])

	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

var publicIDRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidPublicID reports whether s is a lower-case UUID, checked before it
// reaches a uuid column (Postgres rejects malformed input with an error).
func ValidPublicID(s string) bool { return publicIDRe.MatchString(s) }
