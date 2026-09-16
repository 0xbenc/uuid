// Package clip writes a payload to the system clipboard by feeding it to
// the stdin of the first clipboard backend found on PATH.
package clip

import (
	"bytes"
	"errors"
	"fmt"
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
		cmd := exec.Command(path, c.args...)
		cmd.Stdin = bytes.NewReader(payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("clipboard backend %s failed: %v: %s", c.name, err, bytes.TrimSpace(out))
		}
		return nil
	}
	return ErrNoBackend
}
