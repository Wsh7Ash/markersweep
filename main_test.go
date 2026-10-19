package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const genuine = "<<<<<<< HEAD\nours line\n=======\ntheirs line\n>>>>>>> feature\n"
const emptyOurs = "<<<<<<< HEAD\n=======\ntheirs wins\n>>>>>>> feature\n"
const malformed = "<<<<<<< HEAD\nno footer at all\n"

func runCLI(args ...string) (int, string, string) {
	var out, errBuf bytes.Buffer
	code := run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVersionFlag(t *testing.T) {
	code, out, _ := runCLI("--version")
	if code != 0 || out != "markersweep 0.1.0\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestNoPathsIsUsageError(t *testing.T) {
	code, _, errOut := runCLI()
	if code != 2 || !strings.Contains(errOut, "at least one path") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestInvalidStrategy(t *testing.T) {
	code, _, errOut := runCLI("-strategy", "yolo", ".")
	if code != 2 || !strings.Contains(errOut, "invalid -strategy") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestInvalidFormat(t *testing.T) {
	code, _, errOut := runCLI("-format", "xml", ".")
	if code != 2 || !strings.Contains(errOut, "invalid -format") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestMissingPathExits2(t *testing.T) {
	code, _, errOut := runCLI(filepath.Join(t.TempDir(), "nope"))
	if code != 2 || !strings.Contains(errOut, "error:") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestCleanDirectoryExits0(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", "hello\n")
	code, out, _ := runCLI(dir)
	if code != 0 {
		t.Errorf("code=%d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "no conflict markers found") {
		t.Errorf("out:\n%s", out)
	}
}

func TestDryRunWithMarkersExits1(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", genuine)
	code, out, _ := runCLI(dir)
	if code != 1 {
		t.Errorf("code=%d, want 1", code)
	}
	if !strings.Contains(out, "[both]") {
		t.Errorf("out:\n%s", out)
	}
}

func TestAutoWriteLeavesGenuineConflict(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.txt", emptyOurs)
	write(t, dir, "b.txt", genuine)
	code, out, _ := runCLI("-write", dir)
	if code != 1 {
		t.Fatalf("code=%d, want 1 (genuine conflict remains)\n%s", code, out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "theirs wins\n" {
		t.Errorf("a.txt after auto write: %q", data)
	}
	if !strings.Contains(out, "resolved 1, remaining 1") && !strings.Contains(out, "remaining 1") {
		t.Errorf("out missing counts:\n%s", out)
	}
}

func TestTheirsWriteFullyResolves(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.txt", genuine)
	code, out, _ := runCLI("-write", "-strategy", "theirs", dir)
	if code != 0 {
		t.Fatalf("code=%d, want 0\n%s", code, out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "theirs line\n" {
		t.Errorf("a.txt: %q", data)
	}
	// Tree is now clean.
	if code, _, _ := runCLI(dir); code != 0 {
		t.Errorf("re-scan code=%d, want 0", code)
	}
}

func TestUnionWrite(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.txt", genuine)
	code, _, _ := runCLI("-write", "-strategy", "union", dir)
	if code != 0 {
		t.Fatalf("code=%d, want 0", code)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "ours line\ntheirs line\n" {
		t.Errorf("union result: %q", data)
	}
}

func TestMalformedAlwaysRemains(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "broken.txt", malformed)
	for _, strategy := range []string{"auto", "ours", "theirs", "union"} {
		code, out, _ := runCLI("-write", "-strategy", strategy, "-format", "text", dir)
		if code != 1 {
			t.Errorf("strategy %s: code=%d, want 1\n%s", strategy, code, out)
		}
	}
	data, _ := os.ReadFile(path)
	if string(data) != malformed {
		t.Errorf("malformed file was modified: %q", data)
	}
}

func TestJSONOutput(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", genuine)
	code, out, _ := runCLI("-format", "json", dir)
	if code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	var payload struct {
		Tool  string `json:"tool"`
		Stats struct {
			Blocks int `json:"blocks"`
			Both   int `json:"both"`
		} `json:"stats"`
		Files []struct {
			Path   string `json:"path"`
			Blocks []struct {
				Kind   string `json:"kind"`
				Action string `json:"action"`
			} `json:"blocks"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	if payload.Tool != "markersweep" || payload.Stats.Blocks != 1 || payload.Stats.Both != 1 {
		t.Errorf("payload: %+v", payload)
	}
	if len(payload.Files) != 1 || payload.Files[0].Blocks[0].Action != "unresolved" {
		t.Errorf("files: %+v", payload.Files)
	}
}

func TestIgnorePatternSkips(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "notes.conflict", genuine)
	write(t, dir, "a.txt", genuine)
	code, out, _ := runCLI("-ignore", "*.conflict", dir)
	if code != 1 {
		t.Errorf("code=%d, want 1", code)
	}
	if strings.Contains(out, "notes.conflict [both]") || !strings.Contains(out, "notes.conflict\n  skipped: ignored by pattern") {
		t.Errorf("ignored file should be listed as skipped:\n%s", out)
	}
	if !strings.Contains(out, "a.txt") {
		t.Errorf("a.txt missing:\n%s", out)
	}
}

func TestMaxBytesSkipsLargeFile(t *testing.T) {
	dir := t.TempDir()
	big := "<<<<<<< HEAD\n" + strings.Repeat("x", 500) + "\n=======\ny\n>>>>>>> z\n"
	write(t, dir, "big.txt", big)
	code, out, _ := runCLI("-max-bytes", "100", dir)
	if code != 0 {
		t.Errorf("code=%d, want 0 (oversized file skipped)\n%s", code, out)
	}
	if !strings.Contains(out, "larger than") {
		t.Errorf("skip reason missing:\n%s", out)
	}
}
