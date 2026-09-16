package gen

import (
	"encoding/hex"
	"strings"
	"testing"
)

// assertBits checks the version nibble and the 10xx variant bits on the
// raw bytes. A regexp on the string shape cannot tell variant 10xx from
// 11xx, so the bits are asserted explicitly.
func assertBits(t *testing.T, u UUID, version int) {
	t.Helper()
	if got := int(u[6] >> 4); got != version {
		t.Errorf("version nibble = %d, want %d (uuid %s)", got, version, u)
	}
	if got := u[8] & 0xC0; got != 0x80 {
		t.Errorf("variant bits = %#x, want 0x80 (uuid %s)", got, u)
	}
}

func TestNewV4Bits(t *testing.T) {
	seen := make(map[UUID]bool)
	for i := 0; i < 100; i++ {
		u, err := NewV4()
		if err != nil {
			t.Fatalf("NewV4: %v", err)
		}
		assertBits(t, u, 4)
		seen[u] = true
	}
	if len(seen) == 1 {
		t.Error("100 v4 UUIDs are all identical; the entropy source is not random")
	}
}

func TestNewV7PinnedClock(t *testing.T) {
	// 1645557742000 ms = 2022-02-22T19:22:22Z, the RFC 9562 section 7.7
	// example; its 48-bit big-endian form is 017f22e279b0.
	now := func() int64 { return 1645557742000 }
	for i := 0; i < 50; i++ {
		u, err := NewV7(now)
		if err != nil {
			t.Fatalf("NewV7: %v", err)
		}
		assertBits(t, u, 7)
		if got := hex.EncodeToString(u[:6]); got != "017f22e279b0" {
			t.Fatalf("timestamp bytes = %s, want 017f22e279b0 (uuid %s)", got, u)
		}
	}
}

func TestV3KnownAnswer(t *testing.T) {
	ns, err := Namespace("dns")
	if err != nil {
		t.Fatalf("Namespace(dns): %v", err)
	}
	got := NewV3(ns, "www.example.com").String()
	const want = "5df41881-3aed-3515-88a7-2f4a814cf09e"
	if got != want {
		t.Fatalf("v3(dns, www.example.com) = %s, want %s", got, want)
	}
	u, err := ParseUUID(got)
	if err != nil {
		t.Fatal(err)
	}
	assertBits(t, u, 3)
}

func TestV5KnownAnswer(t *testing.T) {
	ns, err := Namespace("dns")
	if err != nil {
		t.Fatalf("Namespace(dns): %v", err)
	}
	got := NewV5(ns, "www.example.com").String()
	const want = "2ed6657d-e927-568b-95e1-2665a8aea6a2"
	if got != want {
		t.Fatalf("v5(dns, www.example.com) = %s, want %s", got, want)
	}
	u, err := ParseUUID(got)
	if err != nil {
		t.Fatal(err)
	}
	assertBits(t, u, 5)
}

func TestV3V5SameInputDiffer(t *testing.T) {
	ns, _ := Namespace("dns")
	if NewV3(ns, "www.example.com") == NewV5(ns, "www.example.com") {
		t.Error("v3 and v5 of the same input are identical; wrong digest or bit setting")
	}
}

func TestBuiltInNamespaces(t *testing.T) {
	cases := map[string]string{
		"dns":  "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"url":  "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		"oid":  "6ba7b812-9dad-11d1-80b4-00c04fd430c8",
		"x500": "6ba7b814-9dad-11d1-80b4-00c04fd430c8",
	}
	for name, want := range cases {
		u, err := Namespace(name)
		if err != nil {
			t.Fatalf("Namespace(%s): %v", name, err)
		}
		if u.String() != want {
			t.Errorf("Namespace(%s) = %s, want %s", name, u, want)
		}
	}
	if u, err := Namespace("DNS"); err != nil || u.String() != "6ba7b810-9dad-11d1-80b4-00c04fd430c8" {
		t.Errorf("Namespace(DNS) = %s, %v; want dns namespace, nil", u, err)
	}
}

func TestNamespaceLiteral(t *testing.T) {
	const literal = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
	u, err := Namespace(literal)
	if err != nil {
		t.Fatalf("Namespace(literal): %v", err)
	}
	if u.String() != literal {
		t.Errorf("Namespace(literal) = %s, want %s", u, literal)
	}
	for _, bad := range []string{"", "nope", "not-a-uuid", "6ba7b811"} {
		if _, err := Namespace(bad); err == nil {
			t.Errorf("Namespace(%q): want error, got nil", bad)
		}
	}
}

func TestFormatStyles(t *testing.T) {
	u, err := ParseUUID("2ed6657d-e927-568b-95e1-2665a8aea6a2")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		s    Style
		want string
	}{
		{"canonical", Style{}, "2ed6657d-e927-568b-95e1-2665a8aea6a2"},
		{"upper", Style{Upper: true}, "2ED6657D-E927-568B-95E1-2665A8AEA6A2"},
		{"braces", Style{Braces: true}, "{2ed6657d-e927-568b-95e1-2665a8aea6a2}"},
		{"no-dashes", Style{NoDashes: true}, "2ed6657de927568b95e12665a8aea6a2"},
		{"urn", Style{URN: true}, "urn:uuid:2ed6657d-e927-568b-95e1-2665a8aea6a2"},
		{"upper+braces", Style{Upper: true, Braces: true}, "{2ED6657D-E927-568B-95E1-2665A8AEA6A2}"},
		{"upper+braces+no-dashes", Style{Upper: true, Braces: true, NoDashes: true}, "{2ED6657DE927568B95E12665A8AEA6A2}"},
		{"urn wins over all styling", Style{URN: true, Upper: true, Braces: true, NoDashes: true}, "urn:uuid:2ed6657d-e927-568b-95e1-2665a8aea6a2"},
	}
	for _, c := range cases {
		if got := Format(u, c.s); got != c.want {
			t.Errorf("%s: Format = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestParseUUID(t *testing.T) {
	const want = "2ed6657d-e927-568b-95e1-2665a8aea6a2"
	good := []string{
		"2ed6657d-e927-568b-95e1-2665a8aea6a2",
		"2ED6657D-E927-568B-95E1-2665A8AEA6A2",
		"2ed6657de927568b95e12665a8aea6a2",
		"{2ed6657d-e927-568b-95e1-2665a8aea6a2}",
		"urn:uuid:2ed6657d-e927-568b-95e1-2665a8aea6a2",
		" URN:UUID:2ED6657DE927568B95E12665A8AEA6A2 ",
	}
	for _, s := range good {
		u, err := ParseUUID(s)
		if err != nil {
			t.Errorf("ParseUUID(%q): %v", s, err)
			continue
		}
		if u.String() != want {
			t.Errorf("ParseUUID(%q) = %s, want %s", s, u, want)
		}
	}
	bad := []string{
		"",
		"xyz",
		"2ed6657d-e927-568b-95e1",
		"gggggggg-gggg-gggg-gggg-gggggggggggg",
		"12345678123412341234123412345678123",
	}
	for _, s := range bad {
		if _, err := ParseUUID(s); err == nil {
			t.Errorf("ParseUUID(%q): want error, got nil", s)
		}
	}
}

func TestGenerateUnsupportedVersion(t *testing.T) {
	for _, v := range []int{0, 1, 2, 6, 8, -1} {
		_, err := Generate(v, UUID{}, "", func() int64 { return 0 })
		if err == nil {
			t.Errorf("Generate(%d): want error, got nil", v)
			continue
		}
		if !strings.Contains(err.Error(), "unsupported version") {
			t.Errorf("Generate(%d) error = %q, want it to mention unsupported version", v, err)
		}
	}
}

func TestGenerateDispatch(t *testing.T) {
	ns, err := Namespace("dns")
	if err != nil {
		t.Fatal(err)
	}
	if u, err := Generate(3, ns, "www.example.com", nil); err != nil || u.String() != "5df41881-3aed-3515-88a7-2f4a814cf09e" {
		t.Errorf("Generate(3) = %s, %v; want the v3 vector", u, err)
	}
	if u, err := Generate(5, ns, "www.example.com", nil); err != nil || u.String() != "2ed6657d-e927-568b-95e1-2665a8aea6a2" {
		t.Errorf("Generate(5) = %s, %v; want the v5 vector", u, err)
	}
	if u, err := Generate(4, UUID{}, "", nil); err != nil {
		t.Errorf("Generate(4): %v", err)
	} else {
		assertBits(t, u, 4)
	}
	if u, err := Generate(7, UUID{}, "", func() int64 { return 1645557742000 }); err != nil {
		t.Errorf("Generate(7): %v", err)
	} else {
		assertBits(t, u, 7)
		if got := hex.EncodeToString(u[:6]); got != "017f22e279b0" {
			t.Errorf("Generate(7) timestamp bytes = %s, want 017f22e279b0", got)
		}
	}
}
