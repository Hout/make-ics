package i18n

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	goweb_i18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Localizer struct {
	bundle *goweb_i18n.Bundle
	loc    *goweb_i18n.Localizer
}

// NewLocalizer loads message files from the provided localesDir and returns a Localizer for the given locale.
func NewLocalizer(localesDir, locale string) (*Localizer, error) {
	bundle := goweb_i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	// load locale files if present
	files, err := filepath.Glob(filepath.Join(localesDir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		_, err := bundle.LoadMessageFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to load message file %s: %w", f, err)
		}
	}
	loc := goweb_i18n.NewLocalizer(bundle, locale)
	return &Localizer{bundle: bundle, loc: loc}, nil
}

func (l *Localizer) T(id string, templateData map[string]interface{}) string {
	msg, _ := l.loc.Localize(&goweb_i18n.LocalizeConfig{MessageID: id, TemplateData: templateData})
	return msg
}

func (l *Localizer) N(id string, pluralCount int, templateData map[string]interface{}) string {
	msg, _ := l.loc.Localize(&goweb_i18n.LocalizeConfig{MessageID: id, PluralCount: &pluralCount, TemplateData: templateData})
	return msg
}
