package conflict

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const singleBlock = `line one
<<<<<<< HEAD
ours content
=======
theirs content
>>>>>>> feature-x
line two
`

func TestParseSingleBlock(t *testing.T) {
	blocks := Parse(singleBlock)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	b := blocks[0]
	if b.Start != 2 || b.End != 6 || b.Separator != 4 {
		t.Errorf("line numbers: start=%d sep=%d end=%d, want 2/4/6", b.Start, b.Separator, b.End)
	}
	if b.OursLabel != "HEAD" || b.TheirsLabel != "feature-x" {
		t.Errorf("labels: %q / %q", b.OursLabel, b.TheirsLabel)
	}
	if len(b.Ours) != 1 || b.Ours[0] != "ours content" {
		t.Errorf("ours: %#v", b.Ours)
	}
	if len(b.Theirs) != 1 || b.Theirs[0] != "theirs content" {
		t.Errorf("theirs: %#v", b.Theirs)
	}
}

func TestParseMultipleBlocks(t *testing.T) {
	content := "<<<<<<< a\n1\n=======\n2\n>>>>>>> a\nmiddle\n<<<<<<< b\nx\n=======\ny\n>>>>>>> b\n"
	blocks := Parse(content)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].Start != 1 || blocks[1].Start != 7 {
		t.Errorf("starts: %d, %d; want 1, 7", blocks[0].Start, blocks[1].Start)
	}
}

func TestParseNoMarkers(t *testing.T) {
	if got := Parse("just a file\nwith ======= inside text but not alone\n"); len(got) != 0 {
		t.Fatalf("got %d blocks, want 0", len(got))
	}
}

func TestParseRequiresSeparatorAloneOnLine(t *testing.T) {
	// Six equals is not a git separator, so the block is unterminated.
	content := "<<<<<<< HEAD\nours\n======\ntheirs\n>>>>>>> feature\n"
	blocks := Parse(content)
	if len(blocks) != 1 || blocks[0].Kind != KindMalformed {
		t.Fatalf("want 1 malformed block, got %#v", blocks)
	}
	if blocks[0].End != 0 {
		t.Errorf("malformed block should have End=0, got %d", blocks[0].End)
	}
}

func TestParseMissingFooterIsMalformed(t *testing.T) {
	content := "<<<<<<< HEAD\nours\n=======\ntheirs\n"
	blocks := Parse(content)
	if len(blocks) != 1 || blocks[0].Kind != KindMalformed {
		t.Fatalf("want malformed, got %#v", blocks)
	}
	if !strings.Contains(blocks[0].Detail, "footer") {
		t.Errorf("detail should mention footer: %q", blocks[0].Detail)
	}
	if blocks[0].Separator == 0 {
		t.Error("separator should still be recorded")
	}
}

func TestParseMissingSeparatorIsMalformed(t *testing.T) {
	content := "<<<<<<< HEAD\nours only\n"
	blocks := Parse(content)
	if len(blocks) != 1 || blocks[0].Kind != KindMalformed {
		t.Fatalf("want malformed, got %#v", blocks)
	}
}

func TestParseNestedHeaders(t *testing.T) {
	content := "<<<<<<< HEAD\n<<<<<<< feature\nbroken\n=======\nok\n>>>>>>> feature\n"
	blocks := Parse(content)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].Kind != KindMalformed {
		t.Errorf("first block should be malformed, got %s", blocks[0].Kind)
	}
	if blocks[1].Kind != "" && blocks[1].Kind != KindBoth {
		t.Errorf("second block kind: %s", blocks[1].Kind)
	}
	if blocks[1].Start != 2 {
		t.Errorf("second block start: %d, want 2", blocks[1].Start)
	}
}

func TestParseIgnoresIndentedMarkers(t *testing.T) {
	content := "    <<<<<<< HEAD\n    =======\n    >>>>>>> x\n"
	if got := Parse(content); len(got) != 0 {
		t.Fatalf("indented markers must not match, got %d blocks", len(got))
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Kind
	}{
		{"identical", "<<<<<<< HEAD\nsame\n=======\nsame\n>>>>>>> x\n", KindIdentical},
		{"empty ours", "<<<<<<< HEAD\n=======\ntheirs line\n>>>>>>> x\n", KindEmptyOurs},
		{"empty theirs", "<<<<<<< HEAD\nours line\n=======\n>>>>>>> x\n", KindEmptyTheirs},
		{"genuine", "<<<<<<< HEAD\nours line\n=======\nother line\n>>>>>>> x\n", KindBoth},
		{"both empty", "<<<<<<< HEAD\n=======\n>>>>>>> x\n", KindIdentical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := Parse(tc.content)
			if len(blocks) != 1 {
				t.Fatalf("blocks: %d", len(blocks))
			}
			Classify(blocks)
			if blocks[0].Kind != tc.want {
				t.Errorf("kind = %s, want %s", blocks[0].Kind, tc.want)
			}
		})
	}
}

func TestClassifyCRLFEmptySide(t *testing.T) {
	content := "<<<<<<< HEAD\r\n=======\r\ntheirs\r\n>>>>>>> x\r\n"
	blocks := Parse(content)
	Classify(blocks)
	if blocks[0].Kind != KindEmptyOurs {
		t.Errorf("kind = %s, want empty-ours (CRLF blank line)", blocks[0].Kind)
	}
}

func TestClassifyIsIdempotentOnMalformed(t *testing.T) {
	blocks := Parse("<<<<<<< HEAD\nno footer here\n")
	Classify(blocks)
	if blocks[0].Kind != KindMalformed {
		t.Errorf("malformed classification was overwritten: %s", blocks[0].Kind)
	}
}

func scanFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScanFindsAndSkips(t *testing.T) {
	root := scanFixture(t, map[string]string{
		"main.go":               singleBlock,
		"clean.txt":             "nothing here\n",
		".git/COMMIT_EDITMSG":   "<<<<<<< HEAD\nhidden\n=======\nhidden2\n>>>>>>> x\n",
		"node_modules/pkg/a.js": "<<<<<<< HEAD\nnope\n=======\nyep\n>>>>>>> x\n",
		"skipme.log":            "<<<<<<< HEAD\nno\n=======\nyes\n>>>>>>> x\n",
	})
	results := Scan(Options{Roots: []string{root}, Ignore: []string{"*.log"}})
	byPath := map[string]FileResult{}
	for _, r := range results {
		byPath[filepath.Base(r.Path)] = r
	}
	if r := byPath["main.go"]; len(r.Blocks) != 1 {
		t.Errorf("main.go blocks: %d, want 1", len(r.Blocks))
	}
	if r := byPath["clean.txt"]; !r.Clean() {
		t.Error("clean.txt should be clean")
	}
	if _, ok := byPath["COMMIT_EDITMSG"]; ok {
		t.Error(".git must be skipped")
	}
	if r, ok := byPath["pkg"]; ok && r.Skipped == "" {
		if len(r.Blocks) > 0 {
			t.Error("node_modules must be skipped")
		}
	}
	for _, r := range results {
		if strings.HasSuffix(r.Path, ".log") && r.Skipped == "" {
			t.Errorf("%s should be skipped by -ignore pattern", r.Path)
		}
	}
}

func TestScanSkipsBinaryAndOversize(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x00, 0x01, '<', '<', '<'}, 0o644); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", 200)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	results := Scan(Options{Roots: []string{root}, MaxBytes: 100})
	for _, r := range results {
		base := filepath.Base(r.Path)
		if base == "bin.dat" && r.Skipped != "binary file" {
			t.Errorf("bin.dat skipped = %q", r.Skipped)
		}
		if base == "big.txt" && r.Skipped == "" {
			t.Error("big.txt should be skipped as oversized")
		}
	}
}

func TestScanExplicitFileBypassesIgnore(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "keep.log")
	if err := os.WriteFile(path, []byte(singleBlock), 0o644); err != nil {
		t.Fatal(err)
	}
	results := Scan(Options{Roots: []string{path}, Ignore: []string{"*.log"}})
	if len(results) != 1 || len(results[0].Blocks) != 1 {
		t.Fatalf("explicit file must be scanned anyway: %#v", results)
	}
}

func TestScanMissingRootSetsError(t *testing.T) {
	results := Scan(Options{Roots: []string{filepath.Join(t.TempDir(), "nope")}})
	if len(results) != 1 || results[0].Err == "" {
		t.Fatalf("want one result with Err set, got %#v", results)
	}
}

func TestScanSortedByPath(t *testing.T) {
	root := scanFixture(t, map[string]string{
		"z.txt": singleBlock, "a.txt": singleBlock, "m/b.txt": singleBlock,
	})
	results := Scan(Options{Roots: []string{root}})
	for i := 1; i < len(results); i++ {
		if results[i-1].Path >= results[i].Path {
			t.Fatalf("not sorted: %q >= %q", results[i-1].Path, results[i].Path)
		}
	}
}
