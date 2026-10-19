# PROJECT SPEC — markersweep

- **Name:** markersweep
- **Category:** cli-tool (developer tooling)
- **Date:** 2026-10-19

## Target user

A developer or CI maintainer who needs to guarantee that no git merge-conflict
markers ever reach a commit.

## Problem

`<<<<<<< HEAD` lines slip into commits when someone resolves a conflict in a
rush, merges with `--no-commit` and forgets, or hand-edits a file that already
contained markers. The result compiles into nonsense or ships broken content,
and `git diff` reviews often miss the markers.

## Solution

An offline Go CLI that scans files/directories for conflict-marker blocks,
classifies each block (identical sides, one side empty, genuine conflict,
malformed), reports them as text or JSON, and can safely resolve the mechanical
classes — with exit codes CI can gate on.

## Core workflow

1. `markersweep .` → report every marker block with line ranges and class.
2. `markersweep -write .` → auto-resolve mechanical blocks (identical /
   one-side-empty), leave genuine conflicts untouched, report what remains.
3. CI: `markersweep -format json .` fails the build (exit 1) while markers exist.

## Must-have features

- Block parser: `<<<<<<<` header (any label), exact `=======` separator,
  `>>>>>>>` footer; multiple blocks per file; malformed detection (unterminated,
  nested headers) that is never auto-resolved.
- Classification: identical / empty-ours / empty-theirs / both / malformed.
- Strategies: `auto` (mechanical only), `ours`, `theirs`, `union` (both sides).
- Dry-run by default; `-write` rewrites files, preserving CRLF and permissions.
- Binary + oversize + `.git`/`node_modules`/vendor/.venv style skipping.
- Text and JSON reports; exit codes 0 clean / 1 markers remain / 2 usage error.

## Stack

Go 1.22+ standard library only; `go vet` / `go test`.

## Test strategy

Table-driven unit tests for parser, classifier, rewriter (incl. CRLF), walker
skip rules, report rendering; `run()`-level tests for exit codes.

## Demo/result strategy

`examples/demo.sh` scans `examples/sample/` (deliberately conflicted fixtures),
prints the report, applies auto resolution into a temp copy, and captures
artifacts under `results/`.

## Safety considerations

Read-only by default; `-write` only touches files the user passed explicitly or
directories under them; never resolves malformed or genuine conflicts under
`auto`.

## Definition of done

`go vet` + `go test` green, demo artifacts regenerate, README commands verified,
no secrets, published to GitHub.
