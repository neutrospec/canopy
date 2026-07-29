package webui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// loadRawLocale reads a locale file's message IDs (top-level keys and
// [table] headers) without go-i18n, so key parity is checked at the source.
func localeIDs(t *testing.T, name string) map[string]bool {
	t.Helper()
	b, err := fs.ReadFile(localesFS, path.Join("locales", name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := toml.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	ids := map[string]bool{}
	for k := range m {
		ids[k] = true
	}
	return ids
}

// M2: every locale defines the same message-ID set.
func TestLocaleKeyParity(t *testing.T) {
	ib, err := loadBundle()
	if err != nil {
		t.Fatal(err)
	}
	if len(ib.langs) < 2 {
		t.Skip("need ≥2 locales")
	}
	ref := "active." + ib.langs[0] + ".toml"
	refIDs := localeIDs(t, ref)
	for _, lang := range ib.langs[1:] {
		other := "active." + lang + ".toml"
		otherIDs := localeIDs(t, other)
		for id := range refIDs {
			if !otherIDs[id] {
				t.Errorf("%s is missing message ID %q (present in %s)", other, id, ref)
			}
		}
		for id := range otherIDs {
			if !refIDs[id] {
				t.Errorf("%s is missing message ID %q (present in %s)", ref, id, other)
			}
		}
	}
}

// M3: unknown Accept-Language falls back to the default locale, and a
// missing message ID returns the ID rather than blank or a panic.
func TestLocaleFallback(t *testing.T) {
	ib, err := loadBundle()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "xx-YY")
	if got := ib.resolveLang(req); got != defaultLang {
		t.Errorf("unknown Accept-Language should resolve to %q, got %q", defaultLang, got)
	}

	loc := ib.localizer(defaultLang)
	if got := localizeString(loc, "no_such_message_id_zzz"); got != "no_such_message_id_zzz" {
		t.Errorf("missing ID should return the ID, got %q", got)
	}
	// A real key must not be blank in any locale.
	for _, lang := range ib.langs {
		if got := localizeString(ib.localizer(lang), "nav_browse"); got == "" {
			t.Errorf("nav_browse blank in %s", lang)
		}
	}
}

// M3 (cont.): the cookie overrides Accept-Language.
func TestLocaleCookieOverride(t *testing.T) {
	ib, err := loadBundle()
	if err != nil {
		t.Fatal(err)
	}
	if !ib.has("en") {
		t.Skip("no en locale")
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "ko")
	req.AddCookie(&http.Cookie{Name: langCookie, Value: "en"})
	if got := ib.resolveLang(req); got != "en" {
		t.Errorf("cookie should win over Accept-Language, got %q", got)
	}
}

// M1: no hardcoded Hangul UI literals remain in converted templates. This
// grows to cover every template as extraction completes; for now it guards
// the converted ones so they can't regress.
var hangul = regexp.MustCompile(`\p{Hangul}`)

func TestNoHardcodedUIStrings(t *testing.T) {
	converted := []string{"base.html", "home.html"}
	for _, name := range converted {
		b, err := fs.ReadFile(assets, "templates/"+name)
		if err != nil {
			t.Fatal(err)
		}
		// Ignore HTML comments (may carry notes); check rendered content.
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "<!--") {
				continue
			}
			if hangul.MatchString(line) {
				t.Errorf("%s:%d has a hardcoded Hangul literal — use {{t \"id\"}}: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
