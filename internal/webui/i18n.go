package webui

// Web UI localization (docs/web-ui-i18n.md): go-i18n message catalogs, one
// per locale (locales/active.<lang>.toml). Only UI chrome is localized —
// wiki page content is the user's markdown and is never translated. Adding a
// locale is adding a file: the embed glob loads every active.*.toml, so no
// code changes (invariant M4).

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

//go:embed locales/*.toml
var localesFS embed.FS

// defaultLang is the built-in fallback when neither the cookie nor
// Accept-Language selects a loaded locale. Korean keeps the current UI
// unchanged for existing users (invariant M3, "no functional change").
const defaultLang = "ko"

const langCookie = "lang"

// i18nBundle loads every embedded catalog and reports the loaded languages
// (for the language selector). Message IDs must match across files (M2).
type i18nBundle struct {
	bundle *i18n.Bundle
	langs  []string // loaded language tags, e.g. ["en","ko"]
}

func loadBundle() (*i18nBundle, error) {
	def, err := language.Parse(defaultLang)
	if err != nil {
		return nil, err
	}
	b := i18n.NewBundle(def)
	b.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	entries, err := fs.ReadDir(localesFS, "locales")
	if err != nil {
		return nil, err
	}
	var langs []string
	for _, e := range entries {
		name := e.Name() // active.<lang>.toml
		if !strings.HasSuffix(name, ".toml") {
			continue
		}
		if _, err := b.LoadMessageFileFS(localesFS, path.Join("locales", name)); err != nil {
			return nil, err
		}
		lang := strings.TrimSuffix(strings.TrimPrefix(name, "active."), ".toml")
		langs = append(langs, lang)
	}
	return &i18nBundle{bundle: b, langs: langs}, nil
}

// localizer builds a per-request localizer from the language preferences,
// most-preferred first: the explicit cookie, then Accept-Language. go-i18n
// matches these against the loaded catalogs, falling back to defaultLang.
func (ib *i18nBundle) localizer(prefs ...string) *i18n.Localizer {
	return i18n.NewLocalizer(ib.bundle, prefs...)
}

// resolveLang picks the display language for a request: cookie > Accept-
// Language > default. Returned as the loaded-locale key the template set is
// stored under (invariant M3: always a valid loaded locale).
func (ib *i18nBundle) resolveLang(r *http.Request) string {
	if c, err := r.Cookie(langCookie); err == nil && ib.has(c.Value) {
		return c.Value
	}
	// Match Accept-Language against loaded locales.
	if al := r.Header.Get("Accept-Language"); al != "" {
		if tags, _, err := language.ParseAcceptLanguage(al); err == nil {
			supported := make([]language.Tag, 0, len(ib.langs))
			for _, l := range ib.langs {
				if t, err := language.Parse(l); err == nil {
					supported = append(supported, t)
				}
			}
			if len(supported) > 0 {
				m := language.NewMatcher(supported)
				_, idx, conf := m.Match(tags...)
				if conf > language.No && idx >= 0 && idx < len(ib.langs) {
					return ib.langs[idx]
				}
			}
		}
	}
	return defaultLang
}

func (ib *i18nBundle) has(lang string) bool {
	for _, l := range ib.langs {
		if l == lang {
			return true
		}
	}
	return false
}

// LangOption is one entry in the language selector: the locale code and
// its endonym (the language's name in its own script — 한국어, English),
// which is how good multilingual sites label the switcher.
type LangOption struct {
	Code string
	Name string
}

// endonym returns a language's self-name, falling back to the code.
func endonym(code string) string {
	if tag, err := language.Parse(code); err == nil {
		if n := display.Self.Name(tag); n != "" {
			return n
		}
	}
	return code
}

// options lists the loaded locales for the selector, endonym-labeled.
func (ib *i18nBundle) options() []LangOption {
	out := make([]LangOption, 0, len(ib.langs))
	for _, code := range ib.langs {
		out = append(out, LangOption{Code: code, Name: endonym(code)})
	}
	return out
}

// tFunc returns the `t` template function bound to a locale: {{t "id"}} for
// a plain label, {{t "id" "Key" val ...}} for messages with TemplateData.
// On any error (missing ID, bad data) it returns the ID — never blank, never
// a panic (invariant M3). Same signature used by Go handlers for dynamic
// strings via localizeString below.
func tFunc(loc *i18n.Localizer) func(id string, kv ...any) string {
	return func(id string, kv ...any) string {
		return localizeString(loc, id, kv...)
	}
}

func localizeString(loc *i18n.Localizer, id string, kv ...any) string {
	cfg := &i18n.LocalizeConfig{MessageID: id}
	if len(kv) > 0 {
		data := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			if k, ok := kv[i].(string); ok {
				data[k] = kv[i+1]
				if k == "Count" {
					data["PluralCount"] = kv[i+1]
				}
			}
		}
		cfg.TemplateData = data
		if pc, ok := data["PluralCount"]; ok {
			cfg.PluralCount = pc
		}
	}
	s, err := loc.Localize(cfg)
	if err != nil || s == "" {
		return id
	}
	return s
}

// i18nFuncMap is the FuncMap additions for a locale's template set.
func i18nFuncMap(loc *i18n.Localizer) template.FuncMap {
	return template.FuncMap{"t": tFunc(loc)}
}
