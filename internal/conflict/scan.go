// Package conflict parses and classifies git merge-conflict marker blocks.
package conflict

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind classifies one conflict-marker block.
type Kind string

const (
	// KindIdentical: both sides are byte-identical.
	KindIdentical Kind = "identical"
	// KindEmptyOurs: the "ours" side is blank — theirs wins.
	KindEmptyOurs Kind = "empty-ours"
	// KindEmptyTheirs: the "theirs" side is blank — ours wins.
	KindEmptyTheirs Kind = "empty-theirs"
	// KindBoth: a genuine conflict needing a human decision.
	KindBoth Kind = "both"
	// KindMalformed: unbalanced markers; never auto-resolved.
	KindMalformed Kind = "malformed"
)

// Block is one conflict-marker region, with 1-based line numbers.
type Block struct {
	Start       int      `json:"start"`        // line of the <<<<<<< header
	Separator   int      `json:"separator"`    // line of ======= (0 if malformed)
	End         int      `json:"end"`          // line of the >>>>>>> footer (0 if malformed)
	OursLabel   string   `json:"ours_label"`   // text after <<<<<<<
	TheirsLabel string   `json:"theirs_label"` // text after >>>>>>>
	Ours        []string `json:"-"`
	Theirs      []string `json:"-"`
	Kind        Kind     `json:"kind"`
	Detail      string   `json:"detail,omitempty"` // malformed reason / classification note
}

// FileResult is the scan outcome for a single file.
type FileResult struct {
	Path      string  `json:"path"`
	Blocks    []Block `json:"blocks"`
	Skipped   string  `json:"skipped,omitempty"` // reason, when not scanned
	Bytes     int     `json:"bytes"`
	Resolved  int     `json:"resolved"`  // blocks resolved by the last Apply
	Remaining int     `json:"remaining"` // blocks still present after Apply
	Err       string  `json:"error,omitempty"`
}

// Clean reports whether the file has no conflict markers at all.
func (f *FileResult) Clean() bool { return len(f.Blocks) == 0 }

func isSeparator(line string) bool {
	line = strings.TrimSuffix(line, "\r")
	if len(line) < 7 {
		return false
	}
	for i := 0; i < len(line); i++ {
		if line[i] != '=' {
			return false
		}
	}
	return true
}

func headerLabel(line string) (label string, ok bool) {
	line = strings.TrimSuffix(line, "\r")
	if !strings.HasPrefix(line, "<<<<<<<") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "<<<<<<<")), true
}

func footerLabel(line string) (label string, ok bool) {
	line = strings.TrimSuffix(line, "\r")
	if !strings.HasPrefix(line, ">>>>>>>") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, ">>>>>>>")), true
}

// Parse splits content into conflict-marker blocks. Text outside blocks is
// ignored. Unbalanced markers produce a KindMalformed block that is never
// resolved automatically.
func Parse(content string) []Block {
	lines := splitLines(content)
	var blocks []Block
	i := 0
	for i < len(lines) {
		oursLabel, isHeader := headerLabel(lines[i])
		if !isHeader {
			i++
			continue
		}
		start := i
		b := Block{Start: start + 1, OursLabel: oursLabel}
		sep := -1
		end := -1
		j := start + 1
		for ; j < len(lines); j++ {
			if isSeparator(lines[j]) {
				sep = j
				continue
			}
			if sep >= 0 {
				if label, ok := footerLabel(lines[j]); ok {
					end = j
					b.TheirsLabel = label
					break
				}
			}
			if _, isNested := headerLabel(lines[j]); isNested {
				// A second header before the footer: everything from the first
				// header is structurally broken.
				break
			}
		}
		switch {
		case sep < 0:
			b.Kind = KindMalformed
			b.Detail = "missing ======= separator before end of file or next header"
			b.End = 0
			b.Separator = 0
			i = start + 1
		case end < 0:
			b.Kind = KindMalformed
			b.Detail = "missing >>>>>>> footer"
			b.Separator = sep + 1
			b.End = 0
			i = start + 1
		default:
			b.Separator = sep + 1
			b.End = end + 1
			b.Ours = lines[start+1 : sep]
			b.Theirs = lines[sep+1 : end]
			i = end + 1
		}
		blocks = append(blocks, b)
	}
	return blocks
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	// Keep \r at end of lines so CRLF files round-trip byte-exactly.
	lines := strings.Split(content, "\n")
	if last := len(lines) - 1; lines[last] == "" {
		lines = lines[:last] // drop the phantom line after a trailing newline
	}
	return lines
}

var defaultIgnores = []string{".git", "node_modules", "vendor", ".venv", "venv",
	"dist", "build", ".next", "target", "__pycache__"}

// Options control walking and scanning.
type Options struct {
	MaxBytes int64
	Ignore   []string // extra base-name patterns to skip
	Roots    []string // files or directories to scan
}

func (o Options) ignoredBase(name string) bool {
	for _, pat := range o.Ignore {
		if ok, _ := filepath.Match(pat, name); ok {
			return true
		}
	}
	return false
}

// isBinary reports whether data looks binary (NUL byte in the first 8 KiB).
func isBinary(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// Scan walks the roots and returns one FileResult per scanned or skipped file,
// sorted by path. Nonexistent roots are returned as results with Err set.
func Scan(opts Options) []FileResult {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 1 << 20
	}
	var results []FileResult
	seen := map[string]bool{}

	addFile := func(path string, explicit bool) {
		clean := filepath.Clean(path)
		if seen[clean] {
			return
		}
		seen[clean] = true
		res := FileResult{Path: clean}
		defer func() { results = append(results, res) }()

		st, err := os.Stat(clean)
		if err != nil {
			res.Err = err.Error()
			return
		}
		if st.IsDir() {
			return
		}
		res.Bytes = int(st.Size())
		if !explicit && opts.ignoredBase(filepath.Base(clean)) {
			res.Skipped = "ignored by pattern"
			return
		}
		if st.Size() > opts.MaxBytes {
			res.Skipped = fmt.Sprintf("larger than %d bytes", opts.MaxBytes)
			return
		}
		data, err := os.ReadFile(clean)
		if err != nil {
			res.Err = err.Error()
			return
		}
		if isBinary(data) {
			res.Skipped = "binary file"
			return
		}
		res.Blocks = Parse(string(data))
		Classify(res.Blocks)
	}

	for _, root := range opts.Roots {
		st, err := os.Stat(root)
		if err != nil {
			results = append(results, FileResult{Path: root, Err: err.Error()})
			continue
		}
		if !st.IsDir() {
			addFile(root, true)
			continue
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				results = append(results, FileResult{Path: path, Err: err.Error()})
				return nil
			}
			name := info.Name()
			if info.IsDir() {
				for _, skip := range defaultIgnores {
					if name == skip {
						return filepath.SkipDir
					}
				}
				if opts.ignoredBase(name) {
					return filepath.SkipDir
				}
				return nil
			}
			addFile(path, false)
			return nil
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results
}
