// Command uuid generates UUIDs (RFC 9562), prints them to stdout, and
// copies them to the clipboard. The no-flag path generates one UUIDv4.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/0xbenc/uuid/internal/clip"
	"github.com/0xbenc/uuid/internal/gen"
)

// version is the binary version string; goreleaser injects it via ldflags.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args, generates, prints, and copies. It returns the process
// exit code: 0 success, 1 generation/clipboard failure, 2 usage error.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uuid", flag.ContinueOnError)
	fs.SetOutput(stderr)

	v := fs.Int("v", 4, "UUID version to generate: 4 (default), 7, 3, 5")
	n := fs.Int("n", 1, "number of UUIDs to generate")
	ns := fs.String("ns", "", "namespace for -v 3/5: dns, url, oid, x500, or a literal UUID")
	name := fs.String("name", "", "name to hash with -ns (required for -v 3/5)")
	noCopy := fs.Bool("no-copy", false, "print only; do not copy to the clipboard")
	upper := fs.Bool("upper", false, "uppercase hex digits")
	braces := fs.Bool("braces", false, "wrap the UUID in curly braces")
	noDashes := fs.Bool("no-dashes", false, "omit the dashes")
	urn := fs.Bool("urn", false, "print urn:uuid:<uuid> (canonical lowercase dashed)")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	fail := func(code int, format string, a ...any) int {
		fmt.Fprintf(stderr, "uuid: "+format+"\n", a...)
		return code
	}

	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	if *urn && (*upper || *braces || *noDashes) {
		return fail(2, "-urn cannot be combined with -upper, -braces, or -no-dashes")
	}
	switch *v {
	case 3, 5:
		if *ns == "" || *name == "" {
			return fail(2, "-ns and -name are required with -v %d", *v)
		}
	case 4, 7:
		if *ns != "" || *name != "" {
			return fail(2, "-ns and -name are only valid with -v 3 or -v 5")
		}
	default:
		return fail(2, "unsupported version: %d (supported: 3, 4, 5, 7)", *v)
	}
	if *n < 1 {
		return fail(2, "-n must be >= 1")
	}

	var nsu gen.UUID
	if *v == 3 || *v == 5 {
		var err error
		nsu, err = gen.Namespace(*ns)
		if err != nil {
			return fail(2, "%v", err)
		}
	}

	style := gen.Style{Upper: *upper, Braces: *braces, NoDashes: *noDashes, URN: *urn}
	now := func() int64 { return time.Now().UnixMilli() }

	parts := make([]string, 0, *n)
	for i := 0; i < *n; i++ {
		u, err := gen.Generate(*v, nsu, *name, now)
		if err != nil {
			return fail(1, "%v", err)
		}
		parts = append(parts, gen.Format(u, style))
	}
	payload := strings.Join(parts, "\n")
	fmt.Fprintln(stdout, payload)

	if !*noCopy {
		if err := clip.Write([]byte(payload)); err != nil {
			return fail(1, "%v", err)
		}
	}
	return 0
}
