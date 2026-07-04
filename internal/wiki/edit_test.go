package wiki

import (
	"strings"
	"testing"
)

const sample = `---
title: "T"
created: 2026-01-01
updated: 2026-01-01
type: concept
tags: [tool]
---

# T

See [[old-page]] and [[old-page|별칭]] and [[old-page#sec]].
Also [[other]].
`

func TestSetFrontmatterField(t *testing.T) {
	out := SetFrontmatterField(sample, "updated", "2026-07-04")
	if !strings.Contains(out, "updated: 2026-07-04") {
		t.Error("updated not bumped")
	}
	if !strings.Contains(out, `title: "T"`) || !strings.Contains(out, "[[old-page]]") {
		t.Error("other content damaged")
	}
	// Insert a field that doesn't exist yet.
	out = SetFrontmatterField(sample, "sources", "[]")
	if !strings.Contains(out, "sources: []") {
		t.Error("missing field not inserted")
	}
}

func TestRewriteLinks(t *testing.T) {
	out := RewriteLinks(sample, "old-page", "new-page")
	for _, want := range []string{"[[new-page]]", "[[new-page|별칭]]", "[[new-page#sec]]", "[[other]]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "old-page") {
		t.Error("old link target survived")
	}
}

func TestStripLinks(t *testing.T) {
	out := StripLinks(sample, "old-page")
	for _, want := range []string{"See old-page and 별칭 and old-page."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "[[other]]") {
		t.Error("unrelated link damaged")
	}
}

func TestReplaceBody(t *testing.T) {
	out := ReplaceBody(sample, "new body\n")
	if !strings.Contains(out, `title: "T"`) || !strings.Contains(out, "new body") || strings.Contains(out, "# T") {
		t.Errorf("body replace wrong:\n%s", out)
	}
}
