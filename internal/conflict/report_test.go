package conflict

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func reportFixture(t *testing.T) []FileResult {
	t.Helper()
	files := []FileResult{
		{Path: "a.txt", Blocks: classified(t, "<<<<<<< HEAD\n=======\nT\n>>>>>>> x\n")},    // empty-ours
		{Path: "b.txt", Blocks: classified(t, "<<<<<<< HEAD\nO\n=======\nT\n>>>>>>> x\n")}, // both
		{Path: "bin.dat", Skipped: "binary file"},
		{Path: "gone.txt", Err: "boom"},
		{Path: "clean.txt"},
	}
	// Simulate a completed auto write on a.txt.
	files[0].Resolved, files[0].Remaining = 1, 0
	files[1].Resolved, files[1].Remaining = 0, 1
	return files
}

func TestComputeStats(t *testing.T) {
	s := ComputeStats(reportFixture(t))
	if s.FilesScanned != 3 { // a.txt, b.txt, clean.txt
		t.Errorf("FilesScanned=%d, want 3", s.FilesScanned)
	}
	if s.FilesSkipped != 1 || s.Errors != 1 {
		t.Errorf("skipped=%d errors=%d, want 1/1", s.FilesSkipped, s.Errors)
	}
	if s.FilesWithMarkers != 2 || s.Blocks != 2 {
		t.Errorf("markers: files=%d blocks=%d, want 2/2", s.FilesWithMarkers, s.Blocks)
	}
	if s.EmptyOurs != 1 || s.Both != 1 {
		t.Errorf("kinds: empty-ours=%d both=%d, want 1/1", s.EmptyOurs, s.Both)
	}
	if s.Resolved != 1 || s.Remaining != 1 {
		t.Errorf("resolved=%d remaining=%d, want 1/1", s.Resolved, s.Remaining)
	}
}

func TestRenderTextWithMarkers(t *testing.T) {
	files := reportFixture(t)
	stats := ComputeStats(files)
	var buf bytes.Buffer
	RenderText(&buf, files, StrategyAuto, stats, true)
	out := buf.String()
	for _, want := range []string{
		"markersweep — merge conflict marker scanner",
		"mode: -write (strategy \"auto\")",
		"a.txt", "empty-ours", "resolved 1, remaining 0",
		"b.txt", "[both]",
		"bin.dat", "skipped: binary file",
		"gone.txt", "error: boom",
		"blocks: 2 total", "strategy \"auto\": 1 resolvable, 1 remain",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "clean.txt") {
		t.Error("clean files must not be listed")
	}
}

func TestRenderTextNoMarkers(t *testing.T) {
	files := []FileResult{{Path: "clean.txt"}, {Path: "other.txt"}}
	stats := ComputeStats(files)
	var buf bytes.Buffer
	RenderText(&buf, files, StrategyAuto, stats, false)
	out := buf.String()
	if !strings.Contains(out, "no conflict markers found in 2 scanned files") {
		t.Errorf("unexpected report:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	files := reportFixture(t)
	stats := ComputeStats(files)
	var buf bytes.Buffer
	if err := RenderJSON(&buf, files, StrategyAuto, stats, true, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Tool     string `json:"tool"`
		Version  string `json:"version"`
		Strategy string `json:"strategy"`
		Written  bool   `json:"written"`
		Stats    Stats  `json:"stats"`
		Files    []struct {
			Path   string `json:"path"`
			Blocks []struct {
				Kind   string `json:"kind"`
				Action string `json:"action"`
			} `json:"blocks"`
		} `json:"files"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if payload.Tool != "markersweep" || payload.Version != "0.1.0" || !payload.Written {
		t.Errorf("header: %+v", payload)
	}
	if payload.Stats.Blocks != 2 {
		t.Errorf("stats blocks: %d", payload.Stats.Blocks)
	}
	if len(payload.Files) != 4 { // a, b, bin, gone — clean.txt omitted
		t.Errorf("files: %d, want 4", len(payload.Files))
	}
	actions := map[string]string{}
	for _, f := range payload.Files {
		for _, b := range f.Blocks {
			actions[f.Path+":"+b.Kind] = b.Action
		}
	}
	if actions["a.txt:empty-ours"] != string(ActionKeepCommon) {
		t.Errorf("a.txt action: %q", actions["a.txt:empty-ours"])
	}
	if actions["b.txt:both"] != string(ActionUnresolved) {
		t.Errorf("b.txt action: %q", actions["b.txt:both"])
	}
}

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	PrintVersion(&buf, "0.1.0")
	if buf.String() != "markersweep 0.1.0\n" {
		t.Errorf("got %q", buf.String())
	}
}
