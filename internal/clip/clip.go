// Package clip writes a payload to the system clipboard by feeding it to
// the stdin of the first clipboard backend found on PATH.
package clip

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ErrNoBackend reports that no clipboard backend was found on PATH.
var ErrNoBackend = errors.New("no clipboard backend found on PATH (tried: wl-copy, xclip, xsel, pbcopy); rerun with -no-copy to print only")

// candidate is a clipboard backend: a command name plus its fixed args.
type candidate struct {
	name string
	args []string
}

// candidates is the preference order; the first name found on PATH wins.
var candidates = []candidate{
	{name: "wl-copy"},
	{name: "xclip", args: []string{"-selection", "clipboard"}},
	{name: "xsel", args: []string{"--input", "--clipboard"}},
	{name: "pbcopy"},
}

// Write sends payload verbatim to the clipboard via the first backend
// found on PATH. Lookup goes through exec.LookPath, so tests can inject
// fakes by prepending a directory to PATH.
func Write(payload []byte) error {
	for _, c := range candidates {
		path, err := exec.LookPath(c.name)
		if err != nil {
			continue
		}
		return run(path, c, payload)
	}
	return ErrNoBackend
}

// run executes one backend with every standard stream backed by a regular
// file instead of a pipe.
//
// A clipboard on X11 and Wayland is owned by a live process: wl-copy and
// xclip fork a daemon that keeps serving the selection until something
// else claims it. That daemon inherits the fds it was handed, so a pipe on
// stdout/stderr stays open for as long as the clipboard holds our UUID —
// and reading it to EOF (what CombinedOutput does) parks the foreground
// `uuid` there too, which is why the shell prompt never came back. Files
// are passed to the child as plain fds with nothing to drain, so Wait
// returns as soon as the foreground backend process exits.
func run(path string, c candidate, payload []byte) error {
	in, err := temp("uuid-clip-in-")
	if err != nil {
		return err
	}
	defer cleanup(in)
	if _, err := in.Write(payload); err != nil {
		return fmt.Errorf("staging clipboard payload: %w", err)
	}
	if _, err := in.Seek(0, 0); err != nil {
		return fmt.Errorf("staging clipboard payload: %w", err)
	}

	log, err := temp("uuid-clip-log-")
	if err != nil {
		return err
	}
	defer cleanup(log)

	cmd := exec.Command(path, c.args...)
	cmd.Stdin = in
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		out, _ := os.ReadFile(log.Name())
		return fmt.Errorf("clipboard backend %s failed: %v: %s", c.name, err, bytes.TrimSpace(out))
	}
	return nil
}

// temp creates a scratch file for one stream of the backend child.
func temp(prefix string) (*os.File, error) {
	f, err := os.CreateTemp("", prefix)
	if err != nil {
		return nil, fmt.Errorf("creating clipboard scratch file: %w", err)
	}
	return f, nil
}

// cleanup closes and removes a scratch file. The child may still hold its
// own fd — unlinking is enough, the bytes go when the last fd closes.
func cleanup(f *os.File) {
	f.Close()
	os.Remove(f.Name())
}
