// Package gen generates and formats UUIDs (RFC 9562). It is pure: no I/O.
package gen

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
)

// UUID is a 128-bit identifier in raw byte form.
type UUID [16]byte

// Clock returns the current Unix time in milliseconds. It is a seam so
// tests can pin the v7 timestamp.
type Clock func() int64

// Style controls how Format renders a UUID. The flags compose freely,
// except that URN forces the canonical lowercase dashed form.
type Style struct {
	Upper    bool // uppercase hex digits
	Braces   bool // wrap in { }
	NoDashes bool // omit the dashes
	URN      bool // urn:uuid:<canonical>; wins over the other three
}

// String renders the UUID in canonical form: lowercase, dashed.
func (u UUID) String() string {
	h := hex.EncodeToString(u[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// Format renders u in the given style.
func Format(u UUID, s Style) string {
	if s.URN {
		return "urn:uuid:" + u.String()
	}
	h := hex.EncodeToString(u[:])
	if s.Upper {
		h = strings.ToUpper(h)
	}
	if !s.NoDashes {
		h = h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
	}
	if s.Braces {
		h = "{" + h + "}"
	}
	return h
}

// ParseUUID accepts a UUID in canonical dashed form, dashless form,
// case-insensitively, with optional surrounding { } braces and an optional
// urn:uuid: prefix.
func ParseUUID(s string) (UUID, error) {
	t := strings.TrimSpace(s)
	if len(t) >= 9 && strings.EqualFold(t[:9], "urn:uuid:") {
		t = t[9:]
	}
	t = strings.Trim(t, "{}")
	t = strings.ReplaceAll(t, "-", "")
	if len(t) != 32 {
		return UUID{}, fmt.Errorf("invalid UUID %q: expected 32 hex characters", s)
	}
	b, err := hex.DecodeString(t)
	if err != nil {
		return UUID{}, fmt.Errorf("invalid UUID %q: %v", s, err)
	}
	var u UUID
	copy(u[:], b)
	return u, nil
}

// builtInNamespaces are the RFC 4122 named namespaces.
var builtInNamespaces = map[string]string{
	"dns":  "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	"url":  "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
	"oid":  "6ba7b812-9dad-11d1-80b4-00c04fd430c8",
	"x500": "6ba7b814-9dad-11d1-80b4-00c04fd430c8",
}

// Namespace resolves id to its 16-byte form. id is one of dns, url, oid,
// x500 (case-insensitive) or a literal UUID string.
func Namespace(id string) (UUID, error) {
	if s, ok := builtInNamespaces[strings.ToLower(strings.TrimSpace(id))]; ok {
		return ParseUUID(s)
	}
	u, err := ParseUUID(id)
	if err != nil {
		return UUID{}, fmt.Errorf("unknown namespace %q (expected dns, url, oid, x500, or a UUID)", id)
	}
	return u, nil
}

// NewV4 returns a random (v4) UUID: 128 bits of crypto/rand with the
// version nibble set to 4 and the variant bits set to 10.
func NewV4() (UUID, error) {
	var u UUID
	if _, err := rand.Read(u[:]); err != nil {
		return UUID{}, fmt.Errorf("uuid v4: %w", err)
	}
	u[6] = (u[6] & 0x0f) | 0x40 // version 4
	u[8] = (u[8] & 0x3f) | 0x80 // variant 10xx
	return u, nil
}

// NewV7 returns a Unix-millisecond-timestamped (v7) UUID. The first 48
// bits are the big-endian millisecond stamp from now; the remainder is
// random. now is a seam so tests can pin the clock.
func NewV7(now Clock) (UUID, error) {
	var u UUID
	if _, err := rand.Read(u[6:]); err != nil {
		return UUID{}, fmt.Errorf("uuid v7: %w", err)
	}
	ts := uint64(now())
	u[0] = byte(ts >> 40)
	u[1] = byte(ts >> 32)
	u[2] = byte(ts >> 24)
	u[3] = byte(ts >> 16)
	u[4] = byte(ts >> 8)
	u[5] = byte(ts)
	u[6] = (u[6] & 0x0f) | 0x70 // version 7
	u[8] = (u[8] & 0x3f) | 0x80 // variant 10xx
	return u, nil
}

// NewV3 returns a name-based (v3, MD5) UUID for namespace ns and name.
func NewV3(ns UUID, name string) UUID {
	return newNamed(md5.New(), ns, name, 3)
}

// NewV5 returns a name-based (v5, SHA-1) UUID for namespace ns and name.
func NewV5(ns UUID, name string) UUID {
	return newNamed(sha1.New(), ns, name, 5)
}

// newNamed hashes namespace-bytes || name, truncates the digest to 16
// bytes, and only then sets the version nibble and variant bits on the
// hash output.
func newNamed(h hash.Hash, ns UUID, name string, version byte) UUID {
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	var u UUID
	copy(u[:], sum[:16])
	u[6] = (u[6] & 0x0f) | (version << 4)
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

// Generate dispatches on the requested version: 3 (MD5), 4 (random),
// 5 (SHA-1), 7 (unix-ms). Any other version is an error.
func Generate(version int, ns UUID, name string, now Clock) (UUID, error) {
	switch version {
	case 3:
		return NewV3(ns, name), nil
	case 4:
		return NewV4()
	case 5:
		return NewV5(ns, name), nil
	case 7:
		return NewV7(now)
	}
	return UUID{}, fmt.Errorf("unsupported version: %d", version)
}
