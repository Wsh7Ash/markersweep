// markersweep scans trees for git merge-conflict markers, reports them, and
// can safely resolve the mechanical classes (identical sides, one side empty).
//
// Exit codes: 0 = clean (or fully resolved), 1 = markers remain,
// 2 = usage or input error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Wsh7Ash/markersweep/internal/conflict"
)

const version = "0.1.0"

type config struct {
	strategy string
	write    bool
	format   string
	maxBytes int64
	ignore   string
	roots    []string
	version  bool
}

func parseFlags(args []string, stderr io.Writer) (*config, bool) {
	fs := flag.NewFlagSet("markersweep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := &config{}
	fs.StringVar(&cfg.strategy, "strategy", conflict.StrategyAuto,
		"resolution strategy: auto|ours|theirs|union")
	fs.BoolVar(&cfg.write, "write", false, "rewrite files (default is a dry-run report)")
	fs.StringVar(&cfg.format, "format", "text", "output format: text|json")
	fs.Int64Var(&cfg.maxBytes, "max-bytes", 1<<20, "skip files larger than this")
	fs.StringVar(&cfg.ignore, "ignore", "", "comma-separated extra base-name patterns to skip")
	fs.BoolVar(&cfg.version, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: markersweep [flags] <path>...")
		fmt.Fprintln(stderr, "scans files/directories for git merge-conflict markers.")
		fmt.Fprintln(stderr, "exit codes: 0 clean, 1 markers remain, 2 usage/input error.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return nil, false
	}
	cfg.roots = fs.Args()
	return cfg, true
}

func validate(cfg *config) error {
	validStrategy := false
	for _, s := range conflict.Strategies() {
		if cfg.strategy == s {
			validStrategy = true
		}
	}
	if !validStrategy {
		return fmt.Errorf("invalid -strategy %q (want %s)",
			cfg.strategy, strings.Join(conflict.Strategies(), "|"))
	}
	if cfg.format != "text" && cfg.format != "json" {
		return fmt.Errorf("invalid -format %q (want text|json)", cfg.format)
	}
	if cfg.maxBytes <= 0 {
		return fmt.Errorf("invalid -max-bytes %d (want > 0)", cfg.maxBytes)
	}
	if cfg.version {
		return nil
	}
	if len(cfg.roots) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	return nil
}

// writeResolved applies the strategy on disk. Returns per-file write errors.
func writeResolved(cfg *config, files []conflict.FileResult, stderr io.Writer) bool {
	hadError := false
	for i := range files {
		f := &files[i]
		if f.Err != "" || len(f.Blocks) == 0 || f.Resolved > 0 {
			continue
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			f.Err = err.Error()
			fmt.Fprintf(stderr, "error: %s\n", f.Err)
			hadError = true
			continue
		}
		st, err := os.Stat(f.Path)
		if err != nil {
			f.Err = err.Error()
			fmt.Fprintf(stderr, "error: %s\n", f.Err)
			hadError = true
			continue
		}
		next, resolved, remaining := conflict.ApplyRewrite(string(data), f.Blocks, cfg.strategy)
		f.Resolved, f.Remaining = resolved, remaining
		if resolved == 0 {
			continue
		}
		if next == string(data) {
			continue
		}
		if err := os.WriteFile(f.Path, []byte(next), st.Mode().Perm()); err != nil {
			f.Err = err.Error()
			fmt.Fprintf(stderr, "error: cannot write %s: %s\n", f.Path, err)
			hadError = true
		}
	}
	return hadError
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, ok := parseFlags(args, stderr)
	if !ok {
		return 2
	}
	if cfg.version {
		conflict.PrintVersion(stdout, version)
		return 0
	}
	if err := validate(cfg); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 2
	}

	var ignore []string
	if cfg.ignore != "" {
		for _, pat := range strings.Split(cfg.ignore, ",") {
			if p := strings.TrimSpace(pat); p != "" {
				ignore = append(ignore, p)
			}
		}
	}

	files := conflict.Scan(conflict.Options{
		MaxBytes: cfg.maxBytes,
		Ignore:   ignore,
		Roots:    cfg.roots,
	})

	// A root that does not exist is a usage error.
	inputError := false
	for _, f := range files {
		if f.Err != "" {
			if _, err := os.Stat(f.Path); err != nil {
				fmt.Fprintf(stderr, "error: %s\n", f.Err)
				inputError = true
			}
		}
	}
	if inputError {
		return 2
	}

	if cfg.write {
		if writeResolved(cfg, files, stderr) {
			return 2
		}
	}

	stats := conflict.ComputeStats(files)
	if cfg.format == "json" {
		if err := conflict.RenderJSON(stdout, files, cfg.strategy, stats, cfg.write, version); err != nil {
			fmt.Fprintf(stderr, "error: %s\n", err)
			return 2
		}
	} else {
		conflict.RenderText(stdout, files, cfg.strategy, stats, cfg.write)
	}

	if stats.Blocks == 0 {
		return 0
	}
	if cfg.write {
		if stats.Remaining > 0 {
			return 1
		}
		return 0
	}
	// Dry run: any markers present in the tree are still present.
	return 1
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
