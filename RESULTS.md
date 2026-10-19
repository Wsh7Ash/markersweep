# Results

Honest record of what was actually executed for markersweep on 2026-10-19.

## Environment

- OS: Windows 11 (MINGW64_NT-10.0-19044, x86_64), MSYS2 MinGW shell
- Runtime: Go 1.27.0 (windows/amd64); module declares `go 1.22`
- Dependency manager: Go modules, standard library only (no external modules)

## Validation performed

| Gate | Result | Evidence |
|---|---|---|
| install | N/A | `go.mod`, stdlib only — no install step |
| build | **PASS** | `go build ./...` → exit 0 |
| format | **PASS** | `gofmt -l .` → no files listed (after `gofmt -w .`) |
| vet | **PASS** | `go vet ./...` → no output, exit 0 |
| unit tests | **PASS** | `go test ./... -count=1` → `ok markersweep` + `ok markersweep/internal/conflict`, exit 0; **52 test cases** (`go test -v | grep -c '^=== RUN'`) |
| lint | **PASS** | `gofmt` + `go vet` are the lint gates for this repo |
| type check | N/A | `go vet` is the type-level gate |
| demo run | **PASS** | `bash examples/demo.sh` → 10/10 `[ok]` assertions, `demo result: ALL-OK`, exit 0 |
| secret scan | **PASS** | grep for `ghp_`/`github_pat_`/`AKIA…`/`sk-…`/private-key headers → no matches |

## Commands executed

```shell
gofmt -l .                                      # (clean, after gofmt -w .)
go vet ./...                                    # exit 0
go build ./...                                  # exit 0
go test ./... -count=1                          # both packages ok, exit 0
go test ./... -count=1 -v | grep -c '^=== RUN'  # 52
bash examples/demo.sh                           # ALL-OK, exit 0
python3 -c "import json; json.load(open('results/report.json'))"   # valid JSON
grep -rIlE '(ghp_|github_pat_|AKIA…|sk-…|BEGIN … PRIVATE KEY)' .   # exit 1 = clean
```

## Observed result

`bash examples/demo.sh` (deterministic, exit codes asserted):

```
[ok] dry-run finds markers (exit 1)
[ok] json report (exit 1)
[ok] auto write leaves genuine conflict (exit 1)
[ok] theirs write fully resolves (exit 0)
[ok] tree clean after resolution (exit 0)
[ok] malformed never auto-resolved (exit 1)
[ok] malformed markers still present (exit 1)
[ok] merge.txt empty-theirs block kept ours content
[ok] merge.txt marker-free after auto
[ok] genuine.md resolved to theirs side (auto refused, theirs wrote)
demo result: ALL-OK
```

Dry-run report against `examples/sample/`: 3 files scanned, 3 blocks
(1 identical, 1 empty-theirs, 1 both) — `strategy "auto": 2 resolvable, 1 remain`.

## Generated artifacts

- `results/report-scan.txt` — dry-run text report over `examples/sample/`
- `results/report.json` — JSON report of the same scan
- `results/report-auto.txt` — `-write` (auto): 2 resolved, genuine left
- `results/report-theirs.txt` — `-write -strategy theirs`: everything resolved
- `results/report-clean.txt` — re-scan after resolution: no markers, exit 0
- `results/report-malformed.txt` — `-write -strategy union` on malformed input
- `results/report-malformed-scan.txt` — malformed markers still present

Regenerate with `bash examples/demo.sh`.

## Defect found and fixed during validation

The first rewrite tests failed with content loss: `ActionKeepCommon` always
returned the *ours* side, so an `empty-ours` block resolved to an **empty**
replacement (deleting the theirs content). Fixed so keep-common keeps whichever
side carries content; three failing tests plus the mixed-block fixture now pin
this behavior.

## Known limitations

- Markers must start at column 0 (git's own rule); indented markers are ignored.
- `=======` inside a conflicted string literal is indistinguishable from a real
  separator — the same ambiguity git itself has.
- Rewrites join kept lines with `\n`; CRLF round-trips because lines retain
  their own `\r`, but mixed-ending blocks can end up mixed (as git writes them).
- No symlink-loop protection beyond `filepath.Walk` defaults; no `.gitignore`
  awareness (use `-ignore`).
- Local run used Go 1.27 with a `go 1.22` module; CI runs on ubuntu-latest.
