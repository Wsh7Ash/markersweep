package conflict

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Stats aggregates a scan (+ optional rewrite) for reporting.
type Stats struct {
	FilesScanned     int `json:"files_scanned"`
	FilesSkipped     int `json:"files_skipped"`
	FilesWithMarkers int `json:"files_with_markers"`
	Blocks           int `json:"blocks"`
	Identical        int `json:"identical"`
	EmptyOurs        int `json:"empty_ours"`
	EmptyTheirs      int `json:"empty_theirs"`
	Both             int `json:"both"`
	Malformed        int `json:"malformed"`
	Resolved         int `json:"resolved"`
	Remaining        int `json:"remaining"`
	Errors           int `json:"errors"`
}

// ComputeStats aggregates per-file results.
func ComputeStats(files []FileResult) Stats {
	var s Stats
	for _, f := range files {
		if f.Err != "" {
			s.Errors++
			continue
		}
		if f.Skipped != "" {
			s.FilesSkipped++
			continue
		}
		s.FilesScanned++
		if len(f.Blocks) > 0 {
			s.FilesWithMarkers++
		}
		s.Resolved += f.Resolved
		s.Remaining += f.Remaining
		for _, b := range f.Blocks {
			s.Blocks++
			switch b.Kind {
			case KindIdentical:
				s.Identical++
			case KindEmptyOurs:
				s.EmptyOurs++
			case KindEmptyTheirs:
				s.EmptyTheirs++
			case KindBoth:
				s.Both++
			case KindMalformed:
				s.Malformed++
			}
		}
	}
	return s
}

func describeBlock(b Block) string {
	labels := ""
	if b.OursLabel != "" || b.TheirsLabel != "" {
		labels = fmt.Sprintf(" %q vs %q", b.OursLabel, b.TheirsLabel)
	}
	span := fmt.Sprintf("L%d-L%d", b.Start, b.End)
	if b.End == 0 {
		span = fmt.Sprintf("L%d-?", b.Start)
	}
	return fmt.Sprintf("  %s [%s]%s — %s", span, b.Kind, labels, b.Detail)
}

// RenderText writes the human-readable report.
func RenderText(w io.Writer, files []FileResult, strategy string, stats Stats, wrote bool) {
	fmt.Fprintln(w, "markersweep — merge conflict marker scanner")
	if wrote {
		fmt.Fprintf(w, "mode: -write (strategy %q)\n", strategy)
	} else {
		fmt.Fprintln(w, "mode: dry run (pass -write to resolve mechanical blocks)")
	}
	fmt.Fprintln(w)
	for _, f := range files {
		switch {
		case f.Err != "":
			fmt.Fprintf(w, "%s\n  error: %s\n", f.Path, f.Err)
		case f.Skipped != "":
			fmt.Fprintf(w, "%s\n  skipped: %s\n", f.Path, f.Skipped)
		case len(f.Blocks) == 0:
			continue
		}
		fmt.Fprintln(w, f.Path)
		for _, b := range f.Blocks {
			fmt.Fprintln(w, describeBlock(b))
		}
		if f.Resolved > 0 || f.Remaining > 0 {
			fmt.Fprintf(w, "  resolved %d, remaining %d\n", f.Resolved, f.Remaining)
		}
	}
	if stats.Blocks == 0 {
		fmt.Fprintf(w, "\nno conflict markers found in %d scanned files\n", stats.FilesScanned)
		return
	}
	resolvable, unresolved := 0, 0
	for _, f := range files {
		r, u := PlanCounts(f.Blocks, strategy)
		resolvable += r
		unresolved += u
	}
	fmt.Fprintf(w, "\nscan: %d files (%d skipped, %d errors), markers in %d files\n",
		stats.FilesScanned, stats.FilesSkipped, stats.Errors, stats.FilesWithMarkers)
	fmt.Fprintf(w, "blocks: %d total — %d identical, %d empty-ours, %d empty-theirs, %d both, %d malformed\n",
		stats.Blocks, stats.Identical, stats.EmptyOurs, stats.EmptyTheirs, stats.Both, stats.Malformed)
	fmt.Fprintf(w, "strategy %q: %d resolvable, %d remain\n", strategy, resolvable, unresolved)
	if wrote {
		fmt.Fprintf(w, "rewritten: %d blocks resolved, %d left in place\n", stats.Resolved, stats.Remaining)
	}
}

// jsonBlock mirrors Block with only the serializable fields.
type jsonBlock struct {
	Start       int    `json:"start"`
	Separator   int    `json:"separator,omitempty"`
	End         int    `json:"end,omitempty"`
	OursLabel   string `json:"ours_label,omitempty"`
	TheirsLabel string `json:"theirs_label,omitempty"`
	Kind        Kind   `json:"kind"`
	Detail      string `json:"detail,omitempty"`
	Action      Action `json:"action"`
}

type jsonFile struct {
	Path      string      `json:"path"`
	Bytes     int         `json:"bytes,omitempty"`
	Skipped   string      `json:"skipped,omitempty"`
	Error     string      `json:"error,omitempty"`
	Blocks    []jsonBlock `json:"blocks,omitempty"`
	Resolved  int         `json:"resolved,omitempty"`
	Remaining int         `json:"remaining,omitempty"`
}

type jsonReport struct {
	Tool     string     `json:"tool"`
	Version  string     `json:"version"`
	Strategy string     `json:"strategy"`
	Written  bool       `json:"written"`
	Stats    Stats      `json:"stats"`
	Files    []jsonFile `json:"files"`
}

// RenderJSON writes the machine-readable report (only files of interest).
func RenderJSON(w io.Writer, files []FileResult, strategy string, stats Stats, wrote bool, version string) error {
	report := jsonReport{
		Tool: "markersweep", Version: version, Strategy: strategy,
		Written: wrote, Stats: stats, Files: []jsonFile{},
	}
	for _, f := range files {
		if f.Err == "" && f.Skipped == "" && len(f.Blocks) == 0 {
			continue
		}
		jf := jsonFile{Path: f.Path, Bytes: f.Bytes, Skipped: f.Skipped,
			Error: f.Err, Resolved: f.Resolved, Remaining: f.Remaining}
		for _, b := range f.Blocks {
			jf.Blocks = append(jf.Blocks, jsonBlock{
				Start: b.Start, Separator: b.Separator, End: b.End,
				OursLabel: b.OursLabel, TheirsLabel: b.TheirsLabel,
				Kind: b.Kind, Detail: b.Detail, Action: ActionFor(b, strategy),
			})
		}
		report.Files = append(report.Files, jf)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// PrintVersion writes the version banner.
func PrintVersion(w io.Writer, version string) {
	fmt.Fprintln(w, "markersweep "+strings.TrimSpace(version))
}
