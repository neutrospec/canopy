// Package mermaid validates mermaid diagram source with the same parser
// the web UI renders with: the vendored mermaid.min.js executed in an
// embedded pure-Go JS runtime (goja). This package owns the bundle;
// webui serves Bundle so linter and renderer can never disagree on a
// diagram's validity (invariant P3).
//
// Environment faults (a shim gap, an unsettled promise, an interrupt)
// are reported separately from diagram faults and must fail open —
// never blame the diagram for the sandbox (invariant P4).
package mermaid

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Bundle is the exact mermaid distribution the web UI serves at
// /static/vendor/mermaid.min.js.
//
//go:embed mermaid.min.js
var Bundle []byte

//go:embed shims.js
var shimsJS string

//go:embed bootstrap.js
var bootstrapJS string

// parseTimeout bounds a single parse; jison backtracking on adversarial
// input can be slow, and an interrupt is an environment verdict, not a
// diagram verdict.
const parseTimeout = 10 * time.Second

// DiagramError is a confirmed syntax error in the diagram itself —
// the renderer would show an error box for this block.
type DiagramError struct {
	Message string // mermaid's own message, usually with a line number
}

func (e *DiagramError) Error() string { return e.Message }

// Validator lazily boots one JS runtime and keeps it for the process.
// Not safe for concurrent use.
type Validator struct {
	mu       sync.Mutex
	vm       *goja.Runtime
	validate goja.Callable
	initErr  error
	booted   bool
}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) boot() error {
	if v.booted {
		return v.initErr
	}
	v.booted = true
	vm := goja.New()
	for _, src := range []string{shimsJS, string(Bundle), bootstrapJS} {
		if _, err := vm.RunString(src); err != nil {
			v.initErr = fmt.Errorf("mermaid sandbox init: %w", err)
			return v.initErr
		}
	}
	fn, ok := goja.AssertFunction(vm.Get("__canopyValidate"))
	if !ok {
		v.initErr = fmt.Errorf("mermaid sandbox init: __canopyValidate missing")
		return v.initErr
	}
	v.vm, v.validate = vm, fn
	return nil
}

// Validate parses src with the real mermaid parser.
//   - bad != nil: the diagram is broken — report or reject it.
//   - err != nil: the validation environment failed — fail open (P4).
//   - both nil: the diagram parses.
func (v *Validator) Validate(src string) (bad *DiagramError, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.boot(); err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			bad, err = nil, fmt.Errorf("mermaid sandbox panic: %v", r)
		}
	}()

	timer := time.AfterFunc(parseTimeout, func() { v.vm.Interrupt("parse timeout") })
	defer timer.Stop()
	defer v.vm.ClearInterrupt()

	res, callErr := v.validate(goja.Undefined(), v.vm.ToValue(src))
	if callErr != nil {
		return nil, fmt.Errorf("mermaid sandbox call: %w", callErr)
	}
	out, ok := res.Export().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("mermaid sandbox: unexpected result %T", res.Export())
	}
	state, _ := out["state"].(string)
	name, _ := out["name"].(string)
	msg, _ := out["msg"].(string)
	switch state {
	case "ok":
		return nil, nil
	case "err":
		if isDiagramFault(name, msg) {
			return &DiagramError{Message: msg}, nil
		}
		return nil, fmt.Errorf("mermaid sandbox: %s", msg)
	default: // "pending": the promise never settled — environment fault
		return nil, fmt.Errorf("mermaid sandbox: parse promise did not settle")
	}
}

// isDiagramFault separates the diagram's own syntax errors from sandbox
// faults. Patterns cover both mermaid parser generations: jison ("Parse
// error on line …", "Lexical error …") and langium ("Parsing failed: …",
// "Lexer error …"), plus type detection (UnknownDiagramError).
func isDiagramFault(name, msg string) bool {
	if name == "UnknownDiagramError" {
		return true
	}
	for _, p := range []string{
		"Parse error",
		"Parsing failed",
		"Lexer error",
		"Lexical error",
		"No diagram type detected",
	} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// Block is one ```mermaid fence found in a markdown body.
type Block struct {
	Source string
	Line   int // 1-based line of the opening fence within the body
}

var mdParser = goldmark.New()

// Blocks extracts ```mermaid fences the way the renderer does — via
// goldmark's fenced-code-block parse — so lint sees exactly the blocks
// the web UI would hand to mermaid.
func Blocks(body string) []Block {
	// Cheap pre-filter before a full markdown parse. Deliberately loose —
	// CommonMark trims info strings, so "``` mermaid" (with a space) is a
	// real diagram fence too; the parse below is the precise judge.
	if !strings.Contains(body, "mermaid") {
		return nil
	}
	src := []byte(body)
	root := mdParser.Parser().Parse(text.NewReader(src))
	var blocks []Block
	ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fcb, ok := n.(*ast.FencedCodeBlock)
		if !ok || fcb.Info == nil || string(fcb.Language(src)) != "mermaid" {
			return ast.WalkContinue, nil
		}
		var buf bytes.Buffer
		for i := 0; i < fcb.Lines().Len(); i++ {
			seg := fcb.Lines().At(i)
			buf.Write(seg.Value(src))
		}
		line := 1 + bytes.Count(src[:fcb.Info.Segment.Start], []byte("\n"))
		blocks = append(blocks, Block{Source: buf.String(), Line: line})
		return ast.WalkContinue, nil
	})
	return blocks
}
