# markersweep

**Find git merge-conflict markers before they ship — and auto-resolve the ones that are safe to resolve.**

## The problem

`<<<<<<< HEAD` lines reach the main branch more often than anyone admits: a
rushed conflict resolution, a `git merge --no-commit` abandoned halfway, a file
pasted from two branches. The file still "looks fine" in review, then breaks the
build, the docs, or a config parser at the worst possible moment. `git diff`
rarely catches it because reviewers skim exactly the marker lines.

## Key features

- **Block parser** — real git markers: `<<<<<<<` header with any label,
  exact `=======` separator, `>>>>>>>` footer; multiple blocks per file;
  unterminated or nested markers are reported as `malformed` and are
  **never** rewritten.
- **Classification** — every block is one of `identical` (both sides byte-equal),
  `empty-ours`, `empty-theirs` (one side blank), `both` (genuine conflict),
  or `malformed`.
- **Four strategies** — `auto` resolves only the mechanical classes; `ours`,
  `theirs`, and `union` (both sides concatenated) are explicit opt-ins for
  genuine conflicts.
- **Dry-run by default** — nothing is touched without `-write`; unresolved and
  malformed blocks pass through byte-for-byte (CRLF and file permissions
  preserved).
- **CI gate** — exit codes: `0` clean, `1` markers remain, `2` usage/input
  error. Text or `-format json` output.
- **Fast and dependency-free** — a single walk/read/parse pass in Go, standard
  library only.

## Example output

```
$ markersweep examples/sample
markersweep — merge conflict marker scanner
mode: dry run (pass -write to resolve mechanical blocks)

examples\sample\genuine.md
  L5-L12 [both] "HEAD" vs "feature/launch" — genuine conflict
examples\sample\merge.txt
  L2-L5 [empty-theirs] "HEAD" vs "feature/flags" — theirs side is empty
  L8-L12 [identical] "HEAD" vs "feature/flags" — both sides are identical

scan: 3 files (0 skipped, 0 errors), markers in 2 files
blocks: 3 total — 1 identical, 0 empty-ours, 1 empty-theirs, 1 both, 0 malformed
strategy "auto": 2 resolvable, 1 remain
```

Exit code 1 — markers are still in the tree. After
`markersweep -write .` (auto), only the genuine conflict remains; that is the
file a human should open.

## Architecture

```
paths → conflict.Scan (walk + binary/size/ignore skip)
      → conflict.Parse / Classify (structural blocks, kinds)
      → conflict.ApplyRewrite (strategy → replacement, unresolved passthrough)
      → report (text/JSON) + exit code
```

Details in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Requirements

- Go 1.22+ to build (single static binary, no runtime deps)

## Installation

```shell
go install github.com/Wsh7Ash/markersweep@latest    # once published
# or from a checkout
go build -o markersweep .
```

## Quick start

```shell
markersweep .                 # dry-run report of the whole repo
markersweep -write .          # auto-resolve mechanical blocks only
markersweep -format json .    # machine-readable, for CI artifacts
```

## Usage

```shell
# gate CI on a marker-free tree (exit 1 while markers exist)
markersweep -format json .

# resolve with an explicit side (you asked for it)
markersweep -write -strategy theirs .

# keep both sides (e.g. .gitignore-style merges)
markersweep -write -strategy union path/to/file

# skip extra patterns, cap file size
markersweep -ignore '*.snap,*.min.js' -max-bytes 262144 .
```

Full flag reference and recipes: [docs/USAGE.md](docs/USAGE.md).

## Configuration

Flags only — no config file:

| Flag | Default | Meaning |
|---|---|---|
| `-strategy` | `auto` | `auto` \| `ours` \| `theirs` \| `union` |
| `-write` | off | rewrite files (default is a dry-run report) |
| `-format` | `text` | `text` or `json` |
| `-max-bytes` | 1048576 | skip files larger than this |
| `-ignore` | – | comma-separated extra base-name patterns |
| `-version` | – | print version |

## Testing

```shell
go vet ./...
go test ./... -count=1
bash examples/demo.sh     # end-to-end demo, writes results/
```

52 test cases cover the parser (including malformed/nested/CRLF), the
classification matrix, every strategy's rewrite output, walker skip rules,
report rendering, and CLI exit codes via the real `run()` entry point.

## Project structure

```
markersweep/
├─ main.go                    # flags, run(), exit codes
├─ internal/conflict/
│  ├─ scan.go                 # parser + walker
│  ├─ resolve.go              # classification + rewrite strategies
│  └─ report.go               # text/JSON reports + stats
├─ examples/
│  ├─ sample/                 # genuine + mechanical + clean fixtures
│  ├─ malformed/              # unterminated-marker fixture
│  └─ demo.sh                 # end-to-end demo, all exit codes asserted
├─ results/                   # generated demo artifacts
└─ docs/                      # PROJECT_SPEC, ARCHITECTURE, USAGE
```

## Limitations

- Markers must start at column 0 (that is also what git produces); markers
  indented inside code blocks are not matched.
- Classification is structural, not semantic: if both sides differ by one
  character, the block is a genuine conflict for a human (or `-strategy`).
- A `=======` line inside a *conflicted* string literal cannot be told apart
  from a real separator — same limitation as git itself.
- Rewriting joins kept lines with `\n`; lines keep their own `\r`, so CRLF
  files round-trip, but a block whose two sides used different line endings
  will end up mixed (exactly as git would write it).

## Roadmap

- `-diff` preview of the resolved file (unified diff before writing)
- Custom per-path strategy rules (e.g. always `union` for `.gitignore`)
- Git alias / pre-commit hook snippet
- SARIF output for code-scanning integrations

## Security notes

markersweep only reads the paths you pass and writes back **only** with
`-write`, only to those paths. It never touches `.git/`, follows no symlinks
outside the given roots' walk, and ships no network code. Run it in CI as a
gate: while it exits 1, markers exist.

## License

MIT — see [LICENSE](LICENSE).
