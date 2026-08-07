package mermaid

import (
	"strings"
	"testing"
)

// One validator for the whole test binary — booting the bundle costs ~250ms.
var v = NewValidator()

func mustValid(t *testing.T, label, src string) {
	t.Helper()
	bad, err := v.Validate(src)
	if err != nil {
		t.Fatalf("%s: environment error: %v", label, err)
	}
	if bad != nil {
		t.Errorf("%s: valid diagram rejected: %s", label, bad.Message)
	}
}

func mustBroken(t *testing.T, label, src string) {
	t.Helper()
	bad, err := v.Validate(src)
	if err != nil {
		t.Fatalf("%s: environment error: %v", label, err)
	}
	if bad == nil {
		t.Errorf("%s: broken diagram accepted", label)
	}
}

// The valid set spans every diagram family the wiki plausibly uses,
// across both parser generations (jison and langium).
func TestValidDiagrams(t *testing.T) {
	for label, src := range map[string]string{
		"flowchart":       "flowchart LR\n  A[\"시작\"] -->|\"예\"| B[\"끝\"]",
		"markdown-string": "flowchart LR\n  A[\"`**굵게** 그리고\n줄바꿈`\"] --> B[\"끝\"]",
		"br-label":        "flowchart LR\n  A[\"첫 줄<br/>둘째 줄\"] --> B",
		"subgraph":        "flowchart TB\n  subgraph G[\"그룹\"]\n    A --> B\n  end",
		"sequence":        "sequenceDiagram\n  A->>B: 안녕",
		"state":           "stateDiagram-v2\n  [*] --> S1",
		"class":           "classDiagram\n  class Animal {\n    +int age\n  }",
		"er":              "erDiagram\n  CUSTOMER ||--o{ ORDER : places",
		"gantt":           "gantt\n  title 일정\n  dateFormat YYYY-MM-DD\n  section A\n  작업1 :a1, 2026-01-01, 3d",
		"pie":             "pie\n  \"개\" : 40\n  \"고양이\" : 60",
		"gitGraph":        "gitGraph\n  commit\n  branch dev\n  commit",
		"mindmap":         "mindmap\n  root((중심))\n    가지1",
		"timeline":        "timeline\n  title 역사\n  2024 : 사건",
	} {
		mustValid(t, label, src)
	}
}

// The broken set is the pitfall list the canopy-wiki skill teaches —
// enforcement and guidance must agree (invariant P1/P2).
func TestBrokenDiagrams(t *testing.T) {
	for label, src := range map[string]string{
		"unbalanced-bracket": "flowchart LR\n  A[시작 --> B[끝]",
		"lowercase-end":      "flowchart LR\n  A --> end",
		"unquoted-paren":     "flowchart LR\n  A[라벨 (괄호)] --> B",
		"typo-header":        "floowchart LR\n  A --> B",
		"empty":              "",
		"broken-sequence":    "sequenceDiagram\n  A->>: nope ->>",
		"broken-pie":         "pie\n  개 : forty",
		"broken-gitgraph":    "gitGraph\n  commmit x",
	} {
		mustBroken(t, label, src)
	}
}

// Error messages must carry mermaid's own diagnostics (line info) so a
// rejection is actionable (P2).
func TestErrorMessageHasLine(t *testing.T) {
	bad, err := v.Validate("flowchart LR\n  A[시작 --> B[끝]")
	if err != nil || bad == nil {
		t.Fatalf("expected diagram error, got bad=%v err=%v", bad, err)
	}
	if !strings.Contains(bad.Message, "line 2") {
		t.Errorf("message lacks line info: %s", bad.Message)
	}
}

// P4: only recognizable parser diagnostics count as diagram faults;
// anything else is the sandbox's problem and must fail open.
func TestFaultClassification(t *testing.T) {
	diagram := map[string][2]string{
		"jison":   {"", "Parse error on line 2:\n..."},
		"langium": {"", "Parsing failed:  Parse error on line 2, column 3"},
		"lexer":   {"", "Parsing failed: Lexer error on line 2, column 3"},
		"lexical": {"", "Lexical error on line 3. Unrecognized text."},
		"detect":  {"UnknownDiagramError", "No diagram type detected matching given configuration"},
	}
	for label, nm := range diagram {
		if !isDiagramFault(nm[0], nm[1]) {
			t.Errorf("%s: should be a diagram fault: %q", label, nm[1])
		}
	}
	env := map[string][2]string{
		"shim-gap":  {"TypeError", "Object has no member 'addHook'"},
		"missing":   {"ReferenceError", "TextEncoder is not defined"},
		"interrupt": {"", "parse timeout"},
	}
	for label, nm := range env {
		if isDiagramFault(nm[0], nm[1]) {
			t.Errorf("%s: must not be blamed on the diagram: %q", label, nm[1])
		}
	}
}

func TestBlocks(t *testing.T) {
	body := "# 제목\n\n산문.\n\n```mermaid\nflowchart LR\n  A --> B\n```\n\n```bash\necho hi\n```\n\n```mermaid\npie\n  \"a\" : 1\n```\n"
	blocks := Blocks(body)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 mermaid blocks, got %d", len(blocks))
	}
	if !strings.HasPrefix(blocks[0].Source, "flowchart LR") {
		t.Errorf("block 0 source wrong: %q", blocks[0].Source)
	}
	if blocks[0].Line != 5 {
		t.Errorf("block 0 line = %d, want 5", blocks[0].Line)
	}
	if !strings.HasPrefix(blocks[1].Source, "pie") {
		t.Errorf("block 1 source wrong: %q", blocks[1].Source)
	}
	if got := Blocks("no fences here"); got != nil {
		t.Errorf("expected nil for fence-free body, got %v", got)
	}
	// Indented (non-fenced) code and inline mentions must not count.
	if got := Blocks("inline ```mermaid`` talk without a real fence"); len(got) != 0 {
		t.Errorf("false positive on inline mention: %v", got)
	}
	// CommonMark trims info strings: "``` mermaid" renders as a diagram
	// in the web UI, so it must be extracted too (renderer parity).
	if got := Blocks("``` mermaid\nflowchart LR\n  A --> B\n```\n"); len(got) != 1 {
		t.Errorf("space-after-fence block missed: %v", got)
	}
}
