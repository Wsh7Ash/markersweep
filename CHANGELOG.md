# Changelog

All notable changes to markersweep are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/).

## [0.1.0] — 2026-10-19

### Added
- Conflict-marker parser: `<<<<<<<` headers (any label), exact `=======`
  separator (CR-tolerant), `>>>>>>>` footer; multiple blocks per file;
  malformed detection (missing separator/footer, nested headers).
- Classification: `identical`, `empty-ours`, `empty-theirs`, `both`, `malformed`.
- Strategies: `auto` (mechanical only), `ours`, `theirs`, `union`.
- Dry-run by default; `-write` rewrites files preserving CRLF, permissions and
  trailing-newline state; unresolved/malformed blocks pass through byte-for-byte.
- Walker with `.git`/`node_modules`/vendor/venv/build skip list, binary
  detection, `-max-bytes`, `-ignore` patterns.
- Text and JSON reports with plan/actual counts.
- Exit codes 0/1/2; 52 test cases; `examples/demo.sh` end-to-end demo.
