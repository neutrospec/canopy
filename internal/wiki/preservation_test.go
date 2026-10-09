package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
)

func TestLinkExtractionAndEditsShareCommonMarkBoundaries(t *testing.T) {
	prose := "[[old]] [[ Concepts/OLD.MD#anchor|alias ]] [[old.md|visible]]\n"
	code := "```txt\n[[old]] [[code-only]]\n```\n" +
		"~~~~\n[[old]] [[code-only]]\n~~~~\n" +
		"````\n```\n[[old]] [[code-only]]\n```\n````\n" +
		"> - ~~~txt\n>   [[old]] [[code-only]]\n>   ~~~\n" +
		"    [[old]] [[code-only]]\n\n" +
		"Inline ``one ` [[old]] [[code-only]]`` and `[[old]] [[code-only]]`.\n"
	header := "---\ntitle: '[[old]]'\nunknown: '[[code-only]]'\n---\n\n"
	src := header + prose + code
	links := ExtractLinks(src)
	if len(links) != 1 || links[0] != "old" {
		t.Fatalf("code/frontmatter counted as links: %v", links)
	}
	got := RewriteLinks(src, "old", "new")
	if !strings.HasPrefix(got, header) || !strings.HasSuffix(got, code) {
		t.Fatalf("changed code/frontmatter:\n%s", got)
	}
	for _, want := range []string{"[[new]]", "[[Concepts/new.MD#anchor|alias ]]", "[[new.md|visible]]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing rewritten target %s", want)
		}
	}
	stripped := StripLinks(src, "old")
	if !strings.HasPrefix(stripped, header) || !strings.HasSuffix(stripped, code) {
		t.Fatal("strip changed code/frontmatter")
	}
	if strings.Contains(strings.TrimPrefix(strings.TrimSuffix(stripped, code), header), "[[") {
		t.Fatal("real links survived strip")
	}
}

func TestPageLinkMoveAndSelfLinks(t *testing.T) {
	src := "[[old#here|self]] [[concepts/old.md#part|path]] [[old.md]] `[[concepts/old.md]]`"
	got := RewritePageLinks(src, "concepts/old.md", "entities/new.md")
	want := "[[new#here|self]] [[entities/new.md#part|path]] [[new.md]] `[[concepts/old.md]]`"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := RewritePageLinks("[[concepts/old.md]] [[old]]", "concepts/old.md", "entities/old.md"); got != "[[entities/old.md]] [[old]]" {
		t.Fatal(got)
	}
}

func TestScalarPatchPreservesUnknownYAMLAndRejectsUnsupported(t *testing.T) {
	src := "---\r\n# header comment\r\ntitle: 'C:\\docs\\file'\r\nupdated: 2026-01-01 # retain this\r\nunknown:\r\n  nested: 9007199254740993 # exact\r\n  block: |\r\n    literal text\r\n---\r\n\r\nbody [[old]]\r\n"
	got, err := SetFrontmatterField(src, "updated", "2026-10-10")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(src, "updated: 2026-01-01", "updated: 2026-10-10", 1)
	if got != want {
		t.Fatalf("unrelated YAML/comments changed:\n%q\n%q", got, want)
	}
	for _, value := range []string{"|\n  2026-01-01", ">-\n  2026-01-01", "&date 2026-01-01", "[one, two]"} {
		bad := "---\ntitle: T\nupdated: " + value + "\nunknown: keep # comment\n---\nbody\n"
		if _, err := SetFrontmatterField(bad, "updated", "2026-10-10"); err == nil {
			t.Errorf("unsupported target accepted: %q", value)
		}
	}
	for _, title := range []string{`C:\new\things`, `quote " and backslash \`, "line\nsecond", "한글"} {
		page := Parse("concepts/safe.md", []byte(NewPageContent(title, "concept", "2026-10-10", nil, nil, "body")))
		if page.FMErr != "" || page.Title != title {
			t.Fatalf("title failed round trip: %q -> %q (%s)", title, page.Title, page.FMErr)
		}
	}
}

func TestFlatLookupAndDuplicateDiagnostics(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "concepts", "nested"), 0o755)
	os.MkdirAll(filepath.Join(root, "entities"), 0o755)
	os.WriteFile(filepath.Join(root, "concepts", "a_b.1.md"), []byte("flat"), 0o644)
	os.WriteFile(filepath.Join(root, "concepts", "nested", "a_b.1.md"), []byte("nested"), 0o644)
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	scan, err := Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Pages) != 1 || !ValidFilename("a_b.1.md") {
		t.Fatal("lawful flat slug rejected")
	}
	for _, name := range []string{"a_b.1", "concepts/a_b.1.md"} {
		if _, err := scan.Find(name); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"concepts/nested/a_b.1.md", "entities/a_b.1.md"} {
		if _, err := scan.Find(name); err == nil {
			t.Fatalf("unsupported path accepted: %s", name)
		}
	}
	os.WriteFile(filepath.Join(root, "entities", "a_b.1.md"), []byte("duplicate"), 0o644)
	if _, err := Scan(w); err == nil || !strings.Contains(err.Error(), "concepts/a_b.1.md") || !strings.Contains(err.Error(), "entities/a_b.1.md") {
		t.Fatalf("duplicate not diagnosed: %v", err)
	}
}

func TestLawfulDotMDSuffixIsAddressableWithoutBanningSlug(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "concepts"), 0o755)
	for _, name := range []string{"foo.md", "foo.md.md"} {
		os.WriteFile(filepath.Join(root, "concepts", name), []byte("source"), 0o644)
	}
	scan, err := Scan(&config.Wiki{Root: root, Cfg: config.Default()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scan.Find("foo.md"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous suffix silently picked: %v", err)
	}
	for _, rel := range []string{"concepts/foo.md", "concepts/foo.md.md"} {
		p, err := scan.Find(rel)
		if err != nil || p.RelPath != rel {
			t.Fatalf("lawful page not addressable: %s %+v %v", rel, p, err)
		}
	}
	src := "[[foo.md.md]] [[foo.md]] [[foo]]"
	if got := StripLinks(src, "foo.md"); got != "foo.md.md [[foo.md]] [[foo]]" {
		t.Fatal(got)
	}
	if got := RewritePageLinks("[[foo]]", "concepts/foo.md", "concepts/new.md.md"); got != "[[new.md.md]]" {
		t.Fatal(got)
	}
}
