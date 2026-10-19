package conflict

import "strings"

// Action is what a strategy does with one block.
type Action string

const (
	ActionKeepCommon Action = "keep-common" // identical sides or the non-empty side
	ActionKeepOurs   Action = "keep-ours"
	ActionKeepTheirs Action = "keep-theirs"
	ActionKeepUnion  Action = "keep-union" // both sides concatenated
	ActionUnresolved Action = "unresolved"
)

// Valid strategies.
const (
	StrategyAuto   = "auto"
	StrategyOurs   = "ours"
	StrategyTheirs = "theirs"
	StrategyUnion  = "union"
)

// Strategies lists the accepted -strategy values.
func Strategies() []string {
	return []string{StrategyAuto, StrategyOurs, StrategyTheirs, StrategyUnion}
}

func sideEmpty(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(strings.TrimSuffix(line, "\r")) != "" {
			return false
		}
	}
	return true
}

func sidesEqual(ours, theirs []string) bool {
	if len(ours) != len(theirs) {
		return false
	}
	for i := range ours {
		if ours[i] != theirs[i] {
			return false
		}
	}
	return true
}

// Classify fills Kind/Detail on every well-formed block. Malformed blocks keep
// the detail Parse assigned. Safe to call more than once.
func Classify(blocks []Block) {
	for i := range blocks {
		b := &blocks[i]
		if b.Kind == KindMalformed {
			continue
		}
		switch {
		case sidesEqual(b.Ours, b.Theirs):
			b.Kind = KindIdentical
			b.Detail = "both sides are identical"
		case sideEmpty(b.Ours):
			b.Kind = KindEmptyOurs
			b.Detail = "ours side is empty"
		case sideEmpty(b.Theirs):
			b.Kind = KindEmptyTheirs
			b.Detail = "theirs side is empty"
		default:
			b.Kind = KindBoth
			b.Detail = "genuine conflict"
		}
	}
}

// ActionFor returns the action a strategy applies to a classified block.
// Malformed blocks are never resolved, whatever the strategy.
func ActionFor(b Block, strategy string) Action {
	if b.Kind == KindMalformed {
		return ActionUnresolved
	}
	if b.Kind == KindIdentical {
		return ActionKeepCommon
	}
	if b.Kind == KindEmptyOurs || b.Kind == KindEmptyTheirs {
		switch strategy {
		case StrategyOurs:
			return ActionKeepOurs
		case StrategyTheirs:
			return ActionKeepTheirs
		default: // auto, union
			return ActionKeepCommon
		}
	}
	// Genuine conflict.
	switch strategy {
	case StrategyOurs:
		return ActionKeepOurs
	case StrategyTheirs:
		return ActionKeepTheirs
	case StrategyUnion:
		return ActionKeepUnion
	default: // auto
		return ActionUnresolved
	}
}

// PlanCounts counts resolvable vs unresolved blocks for a strategy.
func PlanCounts(blocks []Block, strategy string) (resolvable, unresolved int) {
	for _, b := range blocks {
		if ActionFor(b, strategy) == ActionUnresolved {
			unresolved++
		} else {
			resolvable++
		}
	}
	return
}

func replacement(b Block, action Action) []string {
	switch action {
	case ActionKeepOurs:
		return b.Ours
	case ActionKeepTheirs:
		return b.Theirs
	case ActionKeepUnion:
		out := make([]string, 0, len(b.Ours)+len(b.Theirs))
		out = append(out, b.Ours...)
		out = append(out, b.Theirs...)
		return out
	default: // ActionKeepCommon: keep whichever side carries the content
		if b.Kind == KindEmptyOurs {
			return b.Theirs
		}
		return b.Ours
	}
}

// ApplyRewrite rewrites content by applying the strategy to each block.
// It returns the new text, the number of blocks resolved, and the number left
// in place (unresolved genuine conflicts and malformed markers). Unresolved
// blocks are copied through byte-for-byte. Line endings are preserved: lines
// keep their own \r and are joined with \n, and a trailing newline stays.
func ApplyRewrite(content string, blocks []Block, strategy string) (string, int, int) {
	if len(blocks) == 0 {
		return content, 0, 0
	}
	trailing := strings.HasSuffix(content, "\n")
	lines := splitLines(content)
	byStart := make(map[int]int, len(blocks))
	for i, b := range blocks {
		byStart[b.Start] = i
	}

	out := make([]string, 0, len(lines))
	resolved, remaining := 0, 0
	for i := 0; i < len(lines); {
		if idx, ok := byStart[i+1]; ok {
			b := blocks[idx]
			action := ActionFor(b, strategy)
			if b.End == 0 {
				// Malformed: extent unknown; pass the header through untouched.
				out = append(out, lines[i])
				remaining++
				i++
				continue
			}
			if action == ActionUnresolved {
				out = append(out, lines[i:b.End]...)
				remaining++
			} else {
				out = append(out, replacement(b, action)...)
				resolved++
			}
			i = b.End // End is 1-based footer line; next unread index == End
			continue
		}
		out = append(out, lines[i])
		i++
	}
	result := strings.Join(out, "\n")
	if trailing {
		result += "\n"
	}
	return result, resolved, remaining
}
