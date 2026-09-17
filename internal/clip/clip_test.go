package clip

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeBackend writes a shell script named name into dir that copies stdin
// to dest. The scripts use absolute paths only, so tests can set PATH to
// dir alone: that keeps the host's real clipboard backends out of the
// test and makes the candidate order under test exact.
func fakeBackend(t *testing.T, dir, name, dest string) {
	t.Helper()
	script := "#!/bin/sh\n/bin/cat > '" + dest + "'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake %s: %v", name, err)
	}
	t.Setenv("PATH", dir)
}

func TestNoBackend(t *testing.T) {
	// An empty PATH: none of wl-copy, xclip, xsel, pbcopy can be found.
	t.Setenv("PATH", t.TempDir())
	if err := Write([]byte("x")); !errors.Is(err, ErrNoBackend) {
		t.Fatalf("Write err = %v, want ErrNoBackend", err)
	}
}

func TestWriteSendsPayloadToFirstBackend(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "payload.bin")
	fakeBackend(t, dir, "wl-copy", dest)
	if err := Write([]byte("hello-clip")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading clipboard payload: %v", err)
	}
	if string(b) != "hello-clip" {
		t.Fatalf("payload = %q, want %q", b, "hello-clip")
	}
}

func TestFirstCandidateOnPathWins(t *testing.T) {
	dir := t.TempDir()
	wlDest := filepath.Join(dir, "wl.bin")
	xclDest := filepath.Join(dir, "xcl.bin")
	fakeBackend(t, dir, "wl-copy", wlDest)
	if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte("#!/bin/sh\n/bin/cat > '"+xclDest+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write([]byte("order")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if b, err := os.ReadFile(wlDest); err != nil || string(b) != "order" {
		t.Fatalf("wl-copy payload = %q, %v; want %q", b, err, "order")
	}
	if _, err := os.Stat(xclDest); !errors.Is(err, os.ErrNotExist) {
		t.Error("xclip ran even though wl-copy was found first on PATH")
	}
}

func TestBackendChildFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	err := Write([]byte("x"))
	if err == nil {
		t.Fatal("Write: want error when the backend child fails")
	}
	if errors.Is(err, ErrNoBackend) {
		t.Fatal("Write: a failing backend is not the same as no backend")
	}
}

func TestLastCandidate(t *testing.T) {
	// Only a fake pbcopy on PATH: the fourth candidate must be used.
	dir := t.TempDir()
	dest := filepath.Join(dir, "pb.bin")
	fakeBackend(t, dir, "pbcopy", dest)
	if err := Write([]byte("pb")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if b, err := os.ReadFile(dest); err != nil || string(b) != "pb" {
		t.Fatalf("pbcopy payload = %q, %v; want %q", b, err, "pb")
	}
}

// TestBackendDaemonDoesNotBlock is the regression guard for the hang that
// sent `uuid` to the background of the shell instead of back to the
// prompt: real backends fork a daemon that owns the selection and holds
// the inherited fds open, so Write must not wait on anything the daemon
// still has. The fake below mimics that by leaving a long sleep behind
// with all three streams inherited.
func TestBackendDaemonDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "payload.bin")
	script := "#!/bin/sh\n/bin/cat > '" + dest + "'\n/bin/sleep 60 &\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	done := make(chan error, 1)
	go func() { done <- Write([]byte("daemon")) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Write blocked on a backend that forked a daemon holding the inherited fds")
	}
	if b, err := os.ReadFile(dest); err != nil || string(b) != "daemon" {
		t.Fatalf("payload = %q, %v; want %q", b, err, "daemon")
	}
}
