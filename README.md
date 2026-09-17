# uuid

Type `uuid` in a shell to generate one UUIDv4: it is printed to stdout
and copied to your clipboard. Flags open up more of the UUID universe
(RFC 9562). The no-flag path is the product.

## Usage

```
uuid [flags]
```

Examples:

```sh
uuid                                              # v4 to stdout + clipboard
uuid -v 7                                         # v7, unix-millisecond timestamp
uuid -v 5 -ns dns -name www.example.com           # v5 name-based
uuid -v 3 -ns 6ba7b810-9dad-11d1-80b4-00c04fd430c8 -name www.example.com
uuid -n 3                                         # three v4s, newline-joined
uuid -no-copy                                     # print only, no clipboard
uuid -upper -braces -no-dashes                    # styling, freely composable
uuid -urn                                         # urn:uuid:<canonical>
uuid -version                                     # print the binary version
```

Flags:

| Flag        | Meaning                                                                 |
| ----------- | ----------------------------------------------------------------------- |
| `-v N`      | version to generate: `4` (default), `7`, `3`, `5` (v1/v6 unsupported)    |
| `-n N`      | count, default `1`; newline-joined on stdout and in the clipboard        |
| `-ns NAME`  | namespace for v3/v5: `dns`, `url`, `oid`, `x500`, or a literal UUID      |
| `-name S`   | name to hash with `-ns`; required together with `-ns` for v3/v5          |
| `-no-copy`  | print only; skip the clipboard                                          |
| `-upper`    | uppercase hex digits                                                    |
| `-braces`   | wrap in `{ }`                                                          |
| `-no-dashes`| omit the dashes                                                         |
| `-urn`      | `urn:uuid:` form; forces canonical lowercase dashed, cannot combine with the other styling flags |
| `-version`  | print the binary version string and exit                                |

Exit codes: `0` success, `1` generation/clipboard failure, `2` usage error.

Clipboard: the first backend found on PATH wins — `wl-copy`,
`xclip -selection clipboard`, `xsel --input --clipboard`, `pbcopy`. The
payload (styled output, no trailing newline) is written to the
backend's stdin.

## Install

```sh
brew install 0xbenc/tap/uuid
```

coming soon

## Releasing

Push a `vX.Y.Z` tag: goreleaser builds the archives and publishes the GitHub
release, then the workflow renders `packaging/homebrew/uuid.rb.tmpl` from the
release's own `checksums.txt` and pushes it to
[`0xbenc/homebrew-tap`](https://github.com/0xbenc/homebrew-tap) as
`Formula/uuid.rb`.

The tap entry is a **Formula**, not a cask, because `uuid` is installed from it
on Linux as well as macOS and Homebrew casks are macOS-only. goreleaser's own
`brews` support would do this, but it is deprecated and fails `goreleaser
check` as of v2.16, and its replacement (`homebrew_casks`) would drop Linux.

Pushing to the tap needs a `TAP_GITHUB_TOKEN` repository secret with write
access to `0xbenc/homebrew-tap` — the same secret the other 0xbenc tools use.
Prerelease tags (`v1.2.3-rc1`) skip the tap and leave it on the last stable.
