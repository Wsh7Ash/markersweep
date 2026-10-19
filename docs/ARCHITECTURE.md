# Architecture

## Design goals

1. Never damage a file: everything is dry-run by default, and unresolved blocks
   pass through byte-for-byte.
2. Structural honesty: anything the parser does not fully understand is
   `malformed` and is never rewritten.
3. Single-pass performance: one walk, one read, one parse, one rewrite per file.
4. Zero dependencies beyond the Go standard library.

## Pipeline

```text
paths (files or dirs)
   │
   ▼
conflict.Scan            walk; skip .git/node_modules/vendor/.venv/... dirs;
   │                     skip binary (NUL in first 8 KiB), oversize, -ignore
   ▼
conflict.Parse           line scan: <<<<<<< header (label captured),
   │                     exactly-7+-equals separator (CR tolerated), footer
   │                     capture; unterminated/nested → KindMalformed
   ▼
conflict.Classify        identical / empty-ours / empty-theirs / both
   │                     (malformed classification is set once by Parse and
   │                      deliberately not overwritten)
   ▼
report.RenderText/JSON   + (optional) conflict.ApplyRewrite per file
   │                     strategy → Action; unresolved copied through
   ▼
exit code                0 clean · 1 markers remain · 2 usage/input error
```

## Key decisions

### Line-numbered, consuming parser

Blocks are non-overlapping: after a well-formed block ends at its footer the
scan continues after it; after a malformed header the scan resumes **after the
header**, so a second (valid) block later in the file is still found. Line
numbers are 1-based and stable, which is what the report and JSON consumers key
on.

### Why `LF`-tolerant structural matching

Header/separator/footer matching strips a trailing `\r` before testing, so CRLF
files parse correctly, while the *content* lines keep their `\r` byte-for-byte.
Rewriting joins with `\n`, and lines carry their own `\r` — a CRLF file
round-trips exactly (there is a regression test for this).

### `ActionFor`: one table for all strategies

| block kind | auto | ours | theirs | union |
|---|---|---|---|---|
| identical | keep common | keep common | keep common | keep common |
| empty-ours | keep theirs | keep ours | keep theirs | keep theirs |
| empty-theirs | keep ours | keep ours | keep theirs | keep ours |
| both | **unresolved** | keep ours | keep theirs | keep ours+theirs |
| malformed | **unresolved** | **unresolved** | **unresolved** | **unresolved** |

"Keep common" always keeps whichever side actually contains the content, so an
empty side can never wipe a file (this was the first bug the test suite caught).

### Rewrite as line-splice

`ApplyRewrite` maps block start lines, splices replacements into the line
stream, and copies unresolved spans verbatim. Trailing-newline presence is
recorded before splitting and restored after joining, so files without a final
newline keep that property.

### Exit-code contract

- dry-run: markers exist ⇒ exit 1 (the tree is not clean *now*), none ⇒ 0;
- `-write`: anything left after rewriting (genuine conflicts under `auto`,
  malformed blocks under any strategy) ⇒ exit 1, else 0;
- missing roots, invalid flags, unwritable files ⇒ exit 2.

This makes `markersweep .` a one-line CI gate with no extra flags.
