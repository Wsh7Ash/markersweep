#!/usr/bin/env bash
# Deterministic demo for markersweep. Run from anywhere: bash examples/demo.sh
# Verifies every documented exit code and writes results/*.txt + results/report.json.
set -u
cd "$(dirname "$0")/.."

BUILD="$(mktemp -d)"
BIN="$BUILD/markersweep.exe"
go build -o "$BIN" . || { echo "build failed"; exit 2; }
R=results
mkdir -p "$R"
fail=0

expect() { # expect <actual> <want> <label>
  if [ "$1" -ne "$2" ]; then
    echo "[bad] $3: exit $1, want $2"
    fail=1
  else
    echo "[ok] $3 (exit $1)"
  fi
}

set +e
"$BIN" examples/sample > "$R/report-scan.txt";                 expect $? 1 "dry-run finds markers"
"$BIN" -format json examples/sample > "$R/report.json";         expect $? 1 "json report"

TMP="$(mktemp -d)"
cp -r examples/sample "$TMP/tree"
"$BIN" -write "$TMP/tree" > "$R/report-auto.txt";               expect $? 1 "auto write leaves genuine conflict"
"$BIN" -write -strategy theirs "$TMP/tree" > "$R/report-theirs.txt"; expect $? 0 "theirs write fully resolves"
"$BIN" "$TMP/tree" > "$R/report-clean.txt";                     expect $? 0 "tree clean after resolution"

cp -r examples/malformed "$TMP/broken"
"$BIN" -write -strategy union "$TMP/broken" > "$R/report-malformed.txt"; expect $? 1 "malformed never auto-resolved"
"$BIN" "$TMP/broken" > "$R/report-malformed-scan.txt";          expect $? 1 "malformed markers still present"
set -e

if grep -q "debug = false" "$TMP/tree/merge.txt"; then
  echo "[ok] merge.txt empty-theirs block kept ours content"
else
  echo "[bad] merge.txt lost content"; fail=1
fi
if grep -q "^<<<<<<<" "$TMP/tree/merge.txt"; then
  echo "[bad] merge.txt still has markers"; fail=1
else
  echo "[ok] merge.txt marker-free after auto"
fi
if grep -q "Windows service installer" "$TMP/tree/genuine.md"; then
  echo "[ok] genuine.md resolved to theirs side (auto refused, theirs wrote)"
else
  echo "[bad] genuine.md content unexpected"; fail=1
fi

echo "demo result: $([ $fail -eq 0 ] && echo ALL-OK || echo FAILED)"
exit $fail
