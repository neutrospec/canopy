package checkout

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAmbiguousDiscardPreservesBothEditedCopies(t *testing.T) {
	w, _ := testWiki(t)
	raw, err := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Root, "concepts/alpha.md.md"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	short, err := Checkout(w, rescan(t, w), "concepts/alpha.md")
	if err != nil {
		t.Fatal(err)
	}
	long, err := Checkout(w, rescan(t, w), "concepts/alpha.md.md")
	if err != nil {
		t.Fatal(err)
	}
	edit(t, short.Path, "본문 첫 판.", "short unique draft")
	edit(t, long.Path, "본문 첫 판.", "long unique draft")
	a, _ := os.ReadFile(short.Path)
	b, _ := os.ReadFile(long.Path)
	if err := Discard(w, "alpha.md"); err == nil {
		t.Fatal("ambiguous reference discarded a working copy")
	}
	for path, want := range map[string]string{short.Path: hashOf(a), long.Path: hashOf(b)} {
		got, err := os.ReadFile(path)
		if err != nil || hashOf(got) != want {
			t.Fatalf("ambiguous discard lost %s: %v", path, err)
		}
	}
	res, err := CheckinPending(w, rescan(t, w), "concepts/alpha.md.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Complete(w, "concepts/alpha.md.md", res); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(short.Path); err != nil || hashOf(got) != hashOf(a) {
		t.Fatalf("explicit long checkin affected short copy: %v", err)
	}
}

func TestUniqueDottedSlugCheckinAndDeletedOrphanDiscard(t *testing.T) {
	w, _ := testWiki(t)
	page := filepath.Join(w.Root, "concepts/alpha.md")
	if err := os.Rename(page, page+".md"); err != nil {
		t.Fatal(err)
	}
	info, err := Checkout(w, rescan(t, w), "alpha.md")
	if err != nil {
		t.Fatal(err)
	}
	edit(t, info.Path, "본문 첫 판.", "dotted unique draft")
	res, err := CheckinPending(w, rescan(t, w), "alpha.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Complete(w, "alpha.md", res); err != nil {
		t.Fatal(err)
	}
	info, err = Checkout(w, rescan(t, w), "alpha.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(page + ".md"); err != nil {
		t.Fatal(err)
	}
	if err := Discard(w, "alpha.md"); err != nil {
		t.Fatalf("known checkout of deleted page cannot be discarded: %v", err)
	}
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Fatalf("orphan copy still exists: %v", err)
	}
}
