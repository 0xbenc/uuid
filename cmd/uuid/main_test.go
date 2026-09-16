package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xbenc/uuid/internal/gen"
)

func runCapture(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// fakeWLCopy writes a fake wl-copy into dir that copies stdin to dest,
// and sets PATH to dir alone. The scripts use absolute paths only, so
// this keeps the host's real clipboard backends (a live wl-copy hangs
// these tests) out of the run.
func fakeWLCopy(t *testing.T, dir, dest string) {
	t.Helper()
	script := "#!/bin/sh\n/bin/cat > '" + dest + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake wl-copy: %v", err)
	}
	t.Setenv("PATH", dir)
}

func checkV4Line(t *testing.T, i int, line string) {
	t.Helper()
	u, err := gen.ParseUUID(line)
	if err != nil {
		t.Fatalf("stdout line %d %q is not a UUID: %v", i, line, err)
	}
	if u[6]>>4 != 4 {
		t.Fatalf("stdout line %d %q: version nibble = %d, want 4", i, line, u[6]>>4)
	}
	if u[8]&0xC0 != 0x80 {
		t.Fatalf("stdout line %d %q: variant bits = %#x, want 0x80", i, line, u[8]&0xC0)
	}
}

// TestClipboardReceivesNewlineJoinedPayload is the arrival test: a fake
// wl-copy on PATH records exactly what the real code path sends to the
// clipboard, byte for byte.
func TestClipboardReceivesNewlineJoinedPayload(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "clip.bin")
	fakeWLCopy(t, dir, dest)

	code, stdout, stderr := runCapture(t, "-n", "3")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}

	if !strings.HasSuffix(stdout, "\n") || strings.HasSuffix(stdout, "\n\n") {
		t.Fatalf("stdout %q does not end with exactly one newline", stdout)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("stdout has %d lines, want 3: %q", len(lines), stdout)
	}
	for i, line := range lines {
		checkV4Line(t, i, line)
	}

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading clipboard payload: %v", err)
	}
	want := strings.TrimSuffix(stdout, "\n")
	if string(b) != want {
		t.Fatalf("clipboard payload = %q, want %q (newline-joined, no trailing newline)", b, want)
	}
	if bytes.HasSuffix(b, []byte("\n")) {
		t.Fatalf("clipboard payload %q ends with a newline; it must not", b)
	}
}

func TestClipboardStyledNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "clip.bin")
	fakeWLCopy(t, dir, dest)

	code, stdout, stderr := runCapture(t, "-upper", "-braces", "-no-dashes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	want := strings.TrimSuffix(stdout, "\n")
	if !strings.HasPrefix(want, "{") || !strings.HasSuffix(want, "}") || len(want) != 34 {
		t.Fatalf("styled stdout %q, want a 34-char braced uppercase UUID", want)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading clipboard payload: %v", err)
	}
	if string(b) != want {
		t.Fatalf("clipboard payload = %q, want %q", b, want)
	}
}

func TestClipboardUrnNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "clip.bin")
	fakeWLCopy(t, dir, dest)

	code, stdout, stderr := runCapture(t, "-urn")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	want := strings.TrimSuffix(stdout, "\n")
	if !strings.HasPrefix(want, "urn:uuid:") {
		t.Fatalf("stdout %q, want the urn:uuid: prefix", want)
	}
	u, err := gen.ParseUUID(want)
	if err != nil {
		t.Fatalf("urn payload %q is not parseable: %v", want, err)
	}
	if u[6]>>4 != 4 || u[8]&0xC0 != 0x80 {
		t.Fatalf("urn payload %q: bad v4 bits", want)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading clipboard payload: %v", err)
	}
	if string(b) != want {
		t.Fatalf("clipboard payload = %q, want %q", b, want)
	}
}

func TestNoCopySkipsClipboard(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "clip.bin")
	fakeWLCopy(t, dir, dest)

	code, _, stderr := runCapture(t, "-no-copy")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("clipboard backend ran despite -no-copy")
	}
}

func TestNoBackendStillPrintsAndExits1(t *testing.T) {
	// Empty PATH: no clipboard backend exists.
	t.Setenv("PATH", t.TempDir())

	code, stdout, stderr := runCapture(t, "-v", "4")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.HasSuffix(stdout, "\n") || strings.Contains(stdout, "\n\n") {
		t.Fatalf("stdout %q: the UUID must still be printed with one trailing newline", stdout)
	}
	checkV4Line(t, 0, strings.TrimSuffix(stdout, "\n"))
	if stderr == "" {
		t.Error("want an explanatory stderr line about the missing backend")
	}
}

func TestClipboardChildFailureExits1(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	code, stdout, stderr := runCapture(t, "-n", "1")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 for a failing clipboard backend", code)
	}
	if stdout == "" {
		t.Error("the UUID must still be printed when the copy fails")
	}
	if !strings.Contains(stderr, "wl-copy") {
		t.Errorf("stderr %q, want it to name the failing backend", stderr)
	}
}

// TestXclipArgsWhenFirstAvailable proves that when xclip is the first
// candidate on PATH it is invoked as "xclip -selection clipboard" and
// receives the styled payload. PATH is dir alone, so the host's real
// backends cannot interfere.
func TestXclipArgsWhenFirstAvailable(t *testing.T) {
	dir := t.TempDir()
	argsDest := filepath.Join(dir, "args.txt")
	dataDest := filepath.Join(dir, "data.bin")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsDest + "'\n/bin/cat > '" + dataDest + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	code, stdout, stderr := runCapture(t, "-v", "4")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	const wantArgs = "-selection\nclipboard\n"
	if b, err := os.ReadFile(argsDest); err != nil || string(b) != wantArgs {
		t.Fatalf("xclip args = %q, %v; want %q", b, err, wantArgs)
	}
	wantData := strings.TrimSuffix(stdout, "\n")
	if b, err := os.ReadFile(dataDest); err != nil || string(b) != wantData {
		t.Fatalf("xclip data = %q, %v; want %q", b, err, wantData)
	}
}

func TestVersionFlag(t *testing.T) {
	code, stdout, _ := runCapture(t, "-version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSuffix(stdout, "\n") != version {
		t.Fatalf("stdout %q, want %q", stdout, version)
	}
}

func TestCLIKnownAnswers(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-no-copy", "-v", "3", "-ns", "dns", "-name", "www.example.com"}, "5df41881-3aed-3515-88a7-2f4a814cf09e"},
		{[]string{"-no-copy", "-v", "5", "-ns", "dns", "-name", "www.example.com"}, "2ed6657d-e927-568b-95e1-2665a8aea6a2"},
		{[]string{"-no-copy", "-v", "5", "-ns", "DNS", "-name", "www.example.com"}, "2ed6657d-e927-568b-95e1-2665a8aea6a2"},
		{[]string{"-no-copy", "-v", "5", "-ns", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "-name", "www.example.com"}, "2ed6657d-e927-568b-95e1-2665a8aea6a2"},
	}
	for _, c := range cases {
		code, stdout, stderr := runCapture(t, c.args...)
		if code != 0 {
			t.Fatalf("run(%v) exit = %d (stderr: %s)", c.args, code, stderr)
			continue
		}
		if got := strings.TrimSuffix(stdout, "\n"); got != c.want {
			t.Errorf("run(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestCLIV7Timestamp(t *testing.T) {
	code, stdout, stderr := runCapture(t, "-no-copy", "-v", "7")
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	u, err := gen.ParseUUID(strings.TrimSuffix(stdout, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if u[6]>>4 != 7 || u[8]&0xC0 != 0x80 {
		t.Fatalf("v7 bits wrong: %s", u)
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{"-urn", "-upper"},
		{"-urn", "-braces"},
		{"-urn", "-no-dashes"},
		{"-v", "1"},
		{"-v", "2"},
		{"-v", "6"},
		{"-v", "9"},
		{"-v", "3"},
		{"-v", "5", "-ns", "dns"},
		{"-v", "3", "-name", "x"},
		{"-ns", "dns", "-name", "x"},
		{"-v", "4", "-ns", "dns"},
		{"-v", "7", "-ns", "dns", "-name", "x"},
		{"-v", "3", "-ns", "bogus", "-name", "x"},
		{"-n", "0"},
		{"-n", "-3"},
		{"-bogusflag"},
	}
	for _, args := range cases {
		code, _, stderr := runCapture(t, args...)
		if code != 2 {
			t.Errorf("run(%v) exit = %d, want 2 (stderr: %s)", args, code, stderr)
		}
	}
}
