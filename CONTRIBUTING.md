# Contributing to markersweep

Thanks for considering a contribution.

## Development setup

```shell
git clone https://github.com/Wsh7Ash/markersweep.git
cd markersweep
go vet ./...
go test ./... -count=1
bash examples/demo.sh
```

## Ground rules

1. **Never damage user files.** Unresolved and malformed blocks must pass
   through byte-for-byte; any change to that contract needs a test proving it.
2. **Structural honesty.** If the parser does not fully understand a region,
   it must classify it `malformed` rather than guess.
3. **Zero dependencies** outside the Go standard library.
4. **No fabricated results** — `RESULTS.md` and `results/` must reflect commands
   that actually ran.

## Workflow

1. Open an issue describing the bug or idea.
2. Branch from `main`: `git checkout -b fix/short-description`.
3. Add table-driven tests alongside the change (parser cases need fixtures in
   the test, not on disk).
4. `gofmt -w . && go vet ./... && go test ./... -count=1`
5. Update `CHANGELOG.md` and docs if behavior changed.
6. Open a pull request; CI runs vet + tests + the demo on every push.

## Reporting bugs

Include your Go version, the exact command, a minimal file containing the
problematic markers (never real confidential repository content), and the full
output. For rewrite bugs, attach the file content before **and** after.
