# Usage

## Synopsis

```
markersweep [flags] <path>...
```

`<path>...` are files or directories. Directories are walked recursively;
`.git`, `node_modules`, `vendor`, `.venv`, `venv`, `dist`, `build`, `.next`,
`target` and `__pycache__` directories are skipped.

## Exit codes

| code | meaning |
|---|---|
| 0 | clean: no conflict markers (dry-run), or all blocks resolved (`-write`) |
| 1 | markers remain in the tree / could not be resolved |
| 2 | usage or input error (bad flag, missing path, unwritable file) |

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-strategy` | `auto` | `auto` \| `ours` \| `theirs` \| `union` |
| `-write` | off | rewrite files; without it, nothing is modified |
| `-format` | `text` | `text` or `json` |
| `-max-bytes` | `1048576` | skip files larger than this |
| `-ignore` | – | comma-separated base-name patterns to skip (`*.snap,*.min.js`) |
| `-version` | – | print version and exit |

## Block kinds

| kind | meaning | `auto` does |
|---|---|---|
| `identical` | both sides byte-equal (dupes after a bad merge) | keep one copy |
| `empty-ours` | ours side blank | keep theirs |
| `empty-theirs` | theirs side blank | keep ours |
| `both` | genuine conflict | leaves it for you |
| `malformed` | unterminated/nested markers | never touches it |

## Recipes

**CI gate (GitHub Actions):**

```yaml
- name: No conflict markers
  run: go run github.com/Wsh7Ash/markersweep@latest -format json .
```

Any marker in the tree fails the job (exit 1).

**Clean up after a messy merge, then review what is left:**

```shell
markersweep -write .
git diff            # review the mechanical resolutions
$EDITOR $(markersweep -format json . | jq -r '.files[] | select(.blocks) | .path')
```

**Take one side for a whole directory:**

```shell
markersweep -write -strategy theirs src/
```

**Keep both sides (config-style merges):**

```shell
markersweep -write -strategy union .env.example
```

**Ignore generated snapshots:**

```shell
markersweep -ignore '*.snap,*.golden' .
```

## JSON shape (abridged)

```json
{
  "tool": "markersweep",
  "version": "0.1.0",
  "strategy": "auto",
  "written": false,
  "stats": { "files_scanned": 3, "blocks": 3, "both": 1, "malformed": 0, "...": "..." },
  "files": [
    {
      "path": "examples/sample/genuine.md",
      "blocks": [
        { "start": 5, "separator": 8, "end": 12,
          "ours_label": "HEAD", "theirs_label": "feature/launch",
          "kind": "both", "detail": "genuine conflict", "action": "unresolved" }
      ]
    }
  ]
}
```

Only files with markers, skips, or errors are listed; clean files are counted in
`stats.files_scanned`.

## Reading the report

- The per-file block lines give 1-based `Lstart-Lend` spans (the footer line is
  the end), the class, and both labels (`"HEAD" vs "feature-x"`).
- `resolved/remaining` per file is what `-write` did (or would do under the
  chosen strategy).
- The summary line `strategy "auto": N resolvable, M remain` is the plan;
  the `rewritten:` line is the fact after `-write`.
