package conflict

import (
	"strings"
	"testing"
)

func classified(t *testing.T, content string) []Block {
	t.Helper()
	blocks := Parse(content)
	Classify(blocks)
	return blocks
}

func TestActionForMatrix(t *testing.T) {
	cases := []struct {
		content  string
		kind     Kind
		strategy string
		want     Action
	}{
		{"<<<<<<< H\nsame\n=======\nsame\n>>>>>>> x\n", KindIdentical, StrategyAuto, ActionKeepCommon},
		{"<<<<<<< H\nsame\n=======\nsame\n>>>>>>> x\n", KindIdentical, StrategyUnion, ActionKeepCommon},
		{"<<<<<<< H\n=======\nT\n>>>>>>> x\n", KindEmptyOurs, StrategyAuto, ActionKeepCommon},
		{"<<<<<<< H\n=======\nT\n>>>>>>> x\n", KindEmptyOurs, StrategyOurs, ActionKeepOurs},
		{"<<<<<<< H\n=======\nT\n>>>>>>> x\n", KindEmptyOurs, StrategyTheirs, ActionKeepTheirs},
		{"<<<<<<< H\nO\n=======\nT\n>>>>>>> x\n", KindBoth, StrategyAuto, ActionUnresolved},
		{"<<<<<<< H\nO\n=======\nT\n>>>>>>> x\n", KindBoth, StrategyOurs, ActionKeepOurs},
		{"<<<<<<< H\nO\n=======\nT\n>>>>>>> x\n", KindBoth, StrategyTheirs, ActionKeepTheirs},
		{"<<<<<<< H\nO\n=======\nT\n>>>>>>> x\n", KindBoth, StrategyUnion, ActionKeepUnion},
	}
	for _, tc := range cases {
		blocks := Parse(tc.content)
		Classify(blocks)
		if blocks[0].Kind != tc.kind {
			t.Fatalf("fixture kind = %s, want %s", blocks[0].Kind, tc.kind)
		}
		got := ActionFor(blocks[0], tc.strategy)
		if got != tc.want {
			t.Errorf("%s/%s: action = %s, want %s", tc.kind, tc.strategy, got, tc.want)
		}
	}
}

func TestMalformedNeverResolved(t *testing.T) {
	for _, strategy := range Strategies() {
		blocks := Parse("<<<<<<< HEAD\nno separator no footer\n")
		Classify(blocks)
		if got := ActionFor(blocks[0], strategy); got != ActionUnresolved {
			t.Errorf("%s: malformed action = %s, want unresolved", strategy, got)
		}
	}
}

func TestPlanCounts(t *testing.T) {
	content := "<<<<<<< H\n=======\nT\n>>>>>>> x\n" + // empty-ours → auto ok
		"<<<<<<< H\nO\n=======\nT\n>>>>>>> x\n" // both → auto no
	blocks := classified(t, content)
	res, unres := PlanCounts(blocks, StrategyAuto)
	if res != 1 || unres != 1 {
		t.Errorf("auto: resolvable=%d unresolved=%d, want 1/1", res, unres)
	}
	res, unres = PlanCounts(blocks, StrategyTheirs)
	if res != 2 || unres != 0 {
		t.Errorf("theirs: resolvable=%d unresolved=%d, want 2/0", res, unres)
	}
}

func TestApplyRewriteAutoResolvesEmptySide(t *testing.T) {
	content := "head\n<<<<<<< HEAD\n=======\ntheirs line\n>>>>>>> feature\ntail\n"
	blocks := classified(t, content)
	out, resolved, remaining := ApplyRewrite(content, blocks, StrategyAuto)
	if resolved != 1 || remaining != 0 {
		t.Fatalf("resolved=%d remaining=%d, want 1/0", resolved, remaining)
	}
	if strings.Contains(out, "<<<<<<<") || strings.Contains(out, ">>>>>>>") {
		t.Errorf("markers left:\n%s", out)
	}
	if out != "head\ntheirs line\ntail\n" {
		t.Errorf("output:\n%q", out)
	}
}

func TestApplyRewriteAutoLeavesGenuineConflict(t *testing.T) {
	content := "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n"
	blocks := classified(t, content)
	out, resolved, remaining := ApplyRewrite(content, blocks, StrategyAuto)
	if resolved != 0 || remaining != 1 {
		t.Fatalf("resolved=%d remaining=%d, want 0/1", resolved, remaining)
	}
	if out != content {
		t.Errorf("genuine conflict must pass through byte-identical:\ngot  %q\nwant %q", out, content)
	}
}

func TestApplyRewriteStrategies(t *testing.T) {
	content := "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n"
	blocks := classified(t, content)

	out, res, _ := ApplyRewrite(content, blocks, StrategyOurs)
	if out != "ours\n" || res != 1 {
		t.Errorf("ours: %q resolved=%d", out, res)
	}
	out, res, _ = ApplyRewrite(content, blocks, StrategyTheirs)
	if out != "theirs\n" || res != 1 {
		t.Errorf("theirs: %q resolved=%d", out, res)
	}
	out, res, _ = ApplyRewrite(content, blocks, StrategyUnion)
	if out != "ours\ntheirs\n" || res != 1 {
		t.Errorf("union: %q resolved=%d", out, res)
	}
}

func TestApplyRewriteIdenticalNotDuplicatedUnderUnion(t *testing.T) {
	content := "<<<<<<< HEAD\nsame\n=======\nsame\n>>>>>>> feature\n"
	blocks := classified(t, content)
	out, res, rem := ApplyRewrite(content, blocks, StrategyUnion)
	if out != "same\n" || res != 1 || rem != 0 {
		t.Errorf("union on identical: %q res=%d rem=%d", out, res, rem)
	}
}

func TestApplyRewritePreservesCRLFAndUntouchedText(t *testing.T) {
	content := "a\r\n<<<<<<< HEAD\r\nours\r\n=======\r\ntheirs\r\n>>>>>>> feature\r\nb\r\n"
	blocks := classified(t, content)
	out, res, _ := ApplyRewrite(content, blocks, StrategyTheirs)
	if res != 1 {
		t.Fatalf("resolved=%d", res)
	}
	want := "a\r\ntheirs\r\nb\r\n"
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
}

func TestApplyRewritePreservesMissingTrailingNewline(t *testing.T) {
	content := "head\n<<<<<<< HEAD\n=======\nT\n>>>>>>> x\ntail without newline"
	blocks := classified(t, content)
	out, _, _ := ApplyRewrite(content, blocks, StrategyAuto)
	if strings.HasSuffix(out, "\n") {
		t.Errorf("trailing newline was added: %q", out)
	}
	if !strings.HasSuffix(out, "tail without newline") {
		t.Errorf("tail lost: %q", out)
	}
}

func TestApplyRewriteMalformedPassesThrough(t *testing.T) {
	content := "start\n<<<<<<< HEAD\nno footer\n"
	blocks := classified(t, content)
	out, res, rem := ApplyRewrite(content, blocks, StrategyUnion)
	if res != 0 || rem != 1 {
		t.Fatalf("resolved=%d remaining=%d, want 0/1", res, rem)
	}
	if out != content {
		t.Errorf("malformed content must be untouched:\ngot  %q\nwant %q", out, content)
	}
}

func TestApplyRewriteMixedBlocks(t *testing.T) {
	content := strings.Join([]string{
		"1",
		"<<<<<<< HEAD", "same", "=======", "same", ">>>>>>> x",
		"2",
		"<<<<<<< HEAD", "", "=======", "keep", ">>>>>>> y",
		"3",
		"<<<<<<< HEAD", "o", "=======", "t", ">>>>>>> z",
		"4",
	}, "\n") + "\n"
	blocks := classified(t, content)
	out, res, rem := ApplyRewrite(content, blocks, StrategyAuto)
	if res != 2 || rem != 1 {
		t.Fatalf("resolved=%d remaining=%d, want 2/1", res, rem)
	}
	want := "1\nsame\n2\nkeep\n3\n<<<<<<< HEAD\no\n=======\nt\n>>>>>>> z\n4\n"
	if out != want {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestApplyRewriteNoBlocksUnchanged(t *testing.T) {
	out, res, rem := ApplyRewrite("plain\nfile\n", nil, StrategyAuto)
	if out != "plain\nfile\n" || res != 0 || rem != 0 {
		t.Errorf("got %q %d %d", out, res, rem)
	}
}
