package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectNeverRunsInference(t *testing.T) {
	w, st, write := fixture(t)
	write("A current claim.")
	engine := &testEngine{profile: "inspect-fixture", fail: true}
	res, err := InspectCurrentContext(context.Background(), w, st, nil, engine)
	if err != nil || res == nil || res.SemanticReady || res.NeedsReindex != 1 || engine.calls != 0 {
		t.Fatalf("cold inspection: %+v err=%v calls=%d", res, err, engine.calls)
	}
	engine.fail = false
	if _, err := EnsureCurrentContext(context.Background(), w, st, nil, engine, nil); err != nil {
		t.Fatal(err)
	}
	engine.fail = true
	res, err = InspectCurrentContext(context.Background(), w, st, nil, engine)
	if err != nil || !res.SemanticReady || res.NeedsReindex != 0 || engine.calls != 1 {
		t.Fatalf("warm inspection: %+v err=%v calls=%d", res, err, engine.calls)
	}
}

func TestInspectInvalidatesChangedEvidence(t *testing.T) {
	for _, change := range []string{"body", "empty", "deleted", "profile"} {
		t.Run(change, func(t *testing.T) {
			w, st, write := fixture(t)
			write("An obsolete claim.")
			engine := &testEngine{profile: "before"}
			if _, err := EnsureCurrentContext(context.Background(), w, st, nil, engine, nil); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "body":
				write("A corrected claim.")
			case "empty":
				write(" \n\t")
			case "deleted":
				if err := os.Remove(filepath.Join(w.Root, "concepts/a.md")); err != nil {
					t.Fatal(err)
				}
			case "profile":
				engine.profile = "after"
			}
			engine.fail = true
			res, err := InspectCurrentContext(context.Background(), w, st, nil, engine)
			if err != nil || engine.calls != 1 {
				t.Fatalf("inspection ran inference: %+v %v calls=%d", res, err, engine.calls)
			}
			vectors, err := st.PageVectors()
			if err != nil || len(vectors) != 0 {
				t.Fatalf("obsolete vector survived %s: %v %v", change, vectors, err)
			}
			wantNeeds := 1
			if change == "empty" || change == "deleted" {
				wantNeeds = 0
			}
			if res.NeedsReindex != wantNeeds || res.SemanticReady != (wantNeeds == 0) {
				t.Fatalf("incorrect coverage after %s: %+v", change, res)
			}
		})
	}
}

type changedModelEngine struct {
	testEngine
	current bool
}

func (e *changedModelEngine) ProfileCurrent() bool { return e.current }

func TestInspectRejectsReplacedModel(t *testing.T) {
	w, st, write := fixture(t)
	write("A claim.")
	engine := &changedModelEngine{testEngine: testEngine{profile: "loaded"}, current: true}
	if _, err := EnsureCurrentContext(context.Background(), w, st, nil, engine, nil); err != nil {
		t.Fatal(err)
	}
	engine.current = false
	if _, err := InspectCurrentContext(context.Background(), w, st, nil, engine); err == nil {
		t.Fatal("inspection accepted a replaced model")
	}
	if vectors, err := st.PageVectors(); err != nil || len(vectors) != 0 || engine.calls != 1 {
		t.Fatalf("replaced-model vectors survived: %v %v calls=%d", vectors, err, engine.calls)
	}
}
