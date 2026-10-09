package checkout

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/wikilock"
)

func TestOrphanCopiesAreListedAndNeverOverwritten(t *testing.T) {
	for _, fault := range []string{"missing", "corrupt", "unreadable", "unknown", "future", "trailing"} {
		t.Run(fault, func(t *testing.T) {
			w, scan := testWiki(t)
			info, err := Checkout(w, scan, "alpha")
			if err != nil {
				t.Fatal(err)
			}
			edit(t, info.Path, "본문 첫 판.", "unique unrecoverable draft")
			before, _ := os.ReadFile(info.Path)
			_, metaPath := paths(w, "alpha")
			switch fault {
			case "missing":
				os.Remove(metaPath)
			case "corrupt":
				os.WriteFile(metaPath, []byte("{broken"), 0o600)
			case "unreadable":
				os.Remove(metaPath)
				os.Mkdir(metaPath, 0o700)
			case "unknown":
				data, _ := os.ReadFile(metaPath)
				os.WriteFile(metaPath, []byte(strings.Replace(string(data), "{", "{\"unknown\":9007199254740993,", 1)), 0o600)
			case "future":
				data, _ := os.ReadFile(metaPath)
				os.WriteFile(metaPath, []byte(strings.Replace(string(data), "\"version\": 1", "\"version\": 99", 1)), 0o600)
			case "trailing":
				data, _ := os.ReadFile(metaPath)
				os.WriteFile(metaPath, append(data, []byte("{}")...), 0o600)
			}
			if _, err := Checkout(w, scan, "alpha"); err == nil {
				t.Fatalf("orphan %s overwritten", fault)
			}
			after, _ := os.ReadFile(info.Path)
			if hashOf(before) != hashOf(after) {
				t.Fatal("only copy changed")
			}
			open, err := Open(w)
			if err != nil || len(open) != 1 || open[0].Status != "orphan" || open[0].Diagnostic == "" || open[0].Rescue == "" {
				t.Fatalf("orphan hidden: %+v %v", open, err)
			}
			if err := Discard(w, "alpha"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAppliedCheckinCleanupFailureResumesWithoutReapplying(t *testing.T) {
	for _, which := range []string{"copy", "meta"} {
		t.Run(which, func(t *testing.T) {
			w, scan := testWiki(t)
			info, err := Checkout(w, scan, "alpha")
			if err != nil {
				t.Fatal(err)
			}
			edit(t, info.Path, "본문 첫 판.", "applied draft")
			page := filepath.Join(w.Root, "concepts", "alpha.md")
			before, _ := os.ReadFile(page)
			res, err := CheckinPending(w, scan, "alpha", nil)
			if err != nil {
				t.Fatal(err)
			}
			applied, _ := os.ReadFile(page)
			if !res.Applied || res.AppliedHash != hashOf(applied) || hashOf(before) == hashOf(applied) || !res.CleanupPending {
				t.Fatalf("wrong applied result: %+v", res)
			}
			copyPath, metaPath := paths(w, "alpha")
			target := copyPath
			if which == "meta" {
				target = metaPath
			}
			g, err := wikilock.Acquire(context.Background(), w.Root)
			if err != nil {
				t.Fatal(err)
			}
			err = completeLocked(w, "alpha", res, func(path string) error {
				if path == target {
					return fs.ErrPermission
				}
				return os.Remove(path)
			})
			g.Unlock()
			if !errors.Is(err, fs.ErrPermission) || !res.CleanupPending {
				t.Fatalf("cleanup falsely complete: %+v %v", res, err)
			}
			open, err := Open(w)
			if err != nil || len(open) != 1 || open[0].Status != "cleanup_pending" {
				t.Fatalf("pending hidden: %+v %v", open, err)
			}
			again, err := CheckinPending(w, rescan(t, w), "alpha", nil)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(page)
			if !again.Resumed || !again.Applied || again.AppliedHash != res.AppliedHash || hashOf(after) != hashOf(applied) || again.OperationID != res.OperationID {
				t.Fatalf("reapplied or lost checkpoint: %+v", again)
			}
			if err := Complete(w, "alpha", again); err != nil {
				t.Fatal(err)
			}
			if open, _ := Open(w); len(open) != 0 {
				t.Fatal("cleanup did not finish")
			}
		})
	}
}

func TestUnchangedCleanupMetadataOnlyResumes(t *testing.T) {
	w, scan := testWiki(t)
	_, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	res, err := CheckinPending(w, scan, "alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, metaPath := paths(w, "alpha")
	if err := completeLocked(w, "alpha", res, func(path string) error {
		if path == metaPath {
			return fs.ErrPermission
		}
		return os.Remove(path)
	}); err == nil {
		t.Fatal("fault not returned")
	}
	again, err := CheckinPending(w, scan, "alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Unchanged || again.Applied || !again.Resumed {
		t.Fatalf("noop became applied: %+v", again)
	}
	if err := Complete(w, "alpha", again); err != nil {
		t.Fatal(err)
	}
}

func TestCheckinFinalBaseRecheckRejectsValidationRace(t *testing.T) {
	for _, cooperative := range []bool{false, true} {
		t.Run(map[bool]string{false: "external", true: "cooperative"}[cooperative], func(t *testing.T) {
			w, scan := testWiki(t)
			info, err := Checkout(w, scan, "alpha")
			if err != nil {
				t.Fatal(err)
			}
			edit(t, info.Path, "본문 첫 판.", "my draft")
			page := filepath.Join(w.Root, "concepts", "alpha.md")
			foreign := []byte("another writer's complete revision")
			_, err = checkinPending(w, scan, "alpha", nil, func() error {
				if cooperative {
					g, err := wikilock.Acquire(context.Background(), w.Root)
					if err != nil {
						return err
					}
					defer g.Unlock()
				}
				return os.WriteFile(page, foreign, 0o644)
			})
			if err == nil || !strings.Contains(err.Error(), "during validation") {
				t.Fatalf("stale base accepted: %v", err)
			}
			got, _ := os.ReadFile(page)
			copy, _ := os.ReadFile(info.Path)
			if hashOf(got) != hashOf(foreign) || !strings.Contains(string(copy), "my draft") {
				t.Fatal("foreign source or draft lost")
			}
		})
	}
}

func TestRetainedAppliedCheckinRefusesNewExternalEdits(t *testing.T) {
	w, scan := testWiki(t)
	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	edit(t, info.Path, "본문 첫 판.", "applied draft")
	res, err := CheckinPending(w, scan, "alpha", nil)
	if err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(w.Root, res.RelPath)
	foreign := []byte("external newer edit")
	os.WriteFile(page, foreign, 0o644)
	again, err := CheckinPending(w, scan, "alpha", nil)
	if err == nil || again == nil || !again.Applied {
		t.Fatalf("lost known application state: %+v %v", again, err)
	}
	if err := Complete(w, "alpha", res); err == nil {
		t.Fatal("deleted retained copy despite external edit")
	}
	got, _ := os.ReadFile(page)
	if hashOf(got) != hashOf(foreign) {
		t.Fatal("external revision overwritten")
	}
	if _, err := os.Stat(info.Path); err != nil {
		t.Fatal("draft not preserved")
	}
}

func TestCheckinPrepublicationRejectionPreservesOldHash(t *testing.T) {
	w, scan := testWiki(t)
	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	edit(t, info.Path, "updated: 2026-01-01", "updated: |\n  2026-01-01")
	page := filepath.Join(w.Root, "concepts", "alpha.md")
	before, _ := os.ReadFile(page)
	if res, err := CheckinPending(w, scan, "alpha", nil); err == nil || res != nil && res.Applied {
		t.Fatalf("unsupported scalar applied: %+v %v", res, err)
	}
	after, _ := os.ReadFile(page)
	if hashOf(before) != hashOf(after) {
		t.Fatal("prepublication refusal changed old file")
	}
	if _, err := os.Stat(info.Path); err != nil {
		t.Fatal("copy lost")
	}
}
