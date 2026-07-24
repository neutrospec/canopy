package webui

import (
	"strings"
	"testing"
)

func exists(slug string) bool { return slug == "known-page" }

func TestReplaceWikilinks(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"existing", "see [[Known-Page]]", `href="/page/known-page"`},
		{"missing red link", "see [[nope]]", `class="wikilink missing"`},
		{"alias display", "[[known-page|별칭]]", `>별칭</a>`},
		{"anchor kept", "[[known-page#섹션]]", `href="/page/known-page#섹션"`},
		{"path resolves by basename", "[[concepts/known-page]]", `href="/page/known-page"`},
	}
	for _, c := range cases {
		got := replaceWikilinks(c.in, exists)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: %q missing %q in %q", c.name, c.in, c.want, got)
		}
	}
}

func TestWikilinksSkippedInCode(t *testing.T) {
	in := "before [[known-page]]\n\n```\n[[in-fence]]\n```\n\nand `[[inline]]` after"
	got := replaceWikilinks(in, exists)
	if !strings.Contains(got, "[[in-fence]]") || !strings.Contains(got, "`[[inline]]`") {
		t.Fatalf("code regions were rewritten: %q", got)
	}
	if !strings.Contains(got, `href="/page/known-page"`) {
		t.Fatalf("link outside code not rewritten: %q", got)
	}
}

func TestRenderPageProducesHTML(t *testing.T) {
	html, err := RenderPage("# Hi\n\nbody [[known-page]] text", exists)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{"<h1", `<a class="wikilink"`, "body"} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML missing %q: %s", want, s)
		}
	}
}

func TestFirstParagraph(t *testing.T) {
	body := "# Title\n\n> quote\n\nThis is the **real** first paragraph with [[link|a link]].\n\nsecond"
	got := FirstParagraph(body, 100)
	if got != "This is the real first paragraph with link." {
		t.Errorf("got %q", got)
	}
	if long := FirstParagraph(strings.Repeat("가", 200), 10); len([]rune(long)) != 11 {
		t.Errorf("truncation failed: %q", long)
	}
}
