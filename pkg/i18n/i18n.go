package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"

	goweb_i18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales
var localesFS embed.FS

// Localizer wraps go-i18n v2 and exposes simplified T/N accessors.
type Localizer struct {
	loc *goweb_i18n.Localizer
}

// NewLocalizer returns a Localizer for the given locale using the locale files
// embedded in the binary at compile time.
func NewLocalizer(locale string) (*Localizer, error) {
	return newLocalizerFromFS(localesFS, "locales", locale)
}

// newLocalizerFromFS loads message files from fsys/dir and returns a Localizer.
func newLocalizerFromFS(fsys fs.FS, dir, locale string) (*Localizer, error) {
	bundle := goweb_i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read locales dir %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := dir + "/" + e.Name()
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
		if _, err := bundle.ParseMessageFileBytes(data, e.Name()); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
	}
	loc := goweb_i18n.NewLocalizer(bundle, locale)
	return &Localizer{loc: loc}, nil
}

// T returns the localised string for the given message ID, with optional template data.
func (l *Localizer) T(id string, templateData map[string]any) string {
	msg, _ := l.loc.Localize(&goweb_i18n.LocalizeConfig{MessageID: id, TemplateData: templateData})
	return msg
}

// N returns the localised plural string for the given message ID and count.
func (l *Localizer) N(id string, pluralCount int, templateData map[string]any) string {
	msg, _ := l.loc.Localize(&goweb_i18n.LocalizeConfig{MessageID: id, PluralCount: &pluralCount, TemplateData: templateData})
	return msg
}
