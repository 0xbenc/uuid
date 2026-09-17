#!/usr/bin/env python3
"""Render the Homebrew formula for a release.

goreleaser cannot do this for us: its `brews` support is deprecated (it fails
`goreleaser check` as of v2.16) and its replacement, `homebrew_casks`, is
macOS-only — uuid is installed from this tap on Linux too, so the tap keeps a
real Formula. Everything the formula needs beyond the version is in the
release's own checksums.txt, so rendering it is a substitution, not a build.

Usage: render.py VERSION CHECKSUMS_FILE OUT_FILE   (VERSION has no leading v)
"""
import pathlib
import sys

ARCHES = {
    "darwin_arm64": "SHA_DARWIN_ARM64",
    "darwin_amd64": "SHA_DARWIN_AMD64",
    "linux_arm64": "SHA_LINUX_ARM64",
    "linux_amd64": "SHA_LINUX_AMD64",
}


def main() -> int:
    version, checksums, out = sys.argv[1], sys.argv[2], sys.argv[3]
    version = version.lstrip("v")

    sums = {}
    for line in pathlib.Path(checksums).read_text().splitlines():
        if not line.strip():
            continue
        digest, name = line.split()
        sums[name] = digest

    tmpl = (pathlib.Path(__file__).parent / "uuid.rb.tmpl").read_text()
    rendered = tmpl.replace("{{VERSION}}", version)
    for arch, token in ARCHES.items():
        archive = f"uuid_{version}_{arch}.tar.gz"
        if archive not in sums:
            print(f"missing {archive} in {checksums}", file=sys.stderr)
            return 1
        rendered = rendered.replace("{{" + token + "}}", sums[archive])

    if "{{" in rendered:
        print("unsubstituted placeholder left in formula", file=sys.stderr)
        return 1

    pathlib.Path(out).write_text(rendered)
    return 0


if __name__ == "__main__":
    sys.exit(main())
