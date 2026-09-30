// Package localization translates application-owned UI text. It intentionally
// does not translate persisted pattern data, filenames, paths, or rule enums.
package localization

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Preference string

const (
	PreferenceSystem  Preference = "system"
	PreferenceEnglish Preference = "en-US"
	PreferenceSpanish Preference = "es-MX"
)

//go:embed translations/*.json
var translations embed.FS

type Translator struct {
	preference Preference
	localizer  *i18n.Localizer
}

type Language struct {
	Preference Preference
	Name       string
}

// Add a catalog and a registry entry to support another language.
var supported = []Language{
	{PreferenceEnglish, "English (United States)"},
	{PreferenceSpanish, "Español (México)"},
}

func Languages() []Language { return append([]Language(nil), supported...) }

func (preference Preference) Valid() bool {
	if preference == PreferenceSystem {
		return true
	}
	for _, entry := range supported {
		if entry.Preference == preference {
			return true
		}
	}
	return false
}

// Missing fields are handled by the configuration defaults; explicit invalid
// values must not silently turn into the system preference.
func (preference *Preference) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed := Preference(value)
	if !parsed.Valid() {
		return fmt.Errorf("unsupported language %q", value)
	}
	*preference = parsed
	return nil
}

func Resolve(preference Preference, systemLocale string) Preference {
	if preference != PreferenceSystem && preference.Valid() {
		return preference
	}
	parsed, err := language.Parse(systemLocale)
	if err == nil {
		base, _ := parsed.Base()
		for _, entry := range supported {
			if parsed.String() == string(entry.Preference) {
				return entry.Preference
			}
		}
		for _, entry := range supported {
			candidate, _ := language.Make(string(entry.Preference)).Base()
			if base == candidate {
				return entry.Preference
			}
		}
	}
	return PreferenceEnglish
}

func New(preference Preference, systemLocale string) *Translator {
	return newTranslator(preference, systemLocale, translations.ReadFile)
}

func newTranslator(preference Preference, systemLocale string, readFile func(string) ([]byte, error)) *Translator {
	bundle := i18n.NewBundle(language.AmericanEnglish)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, entry := range supported {
		name := "translations/" + string(entry.Preference) + ".json"
		data, err := readFile(name)
		if err != nil {
			panic(fmt.Sprintf("read embedded translations %q: %v", name, err))
		}
		if _, err := bundle.ParseMessageFileBytes(data, name); err != nil {
			panic(fmt.Sprintf("parse embedded translations %q: %v", name, err))
		}
	}
	resolved := Resolve(preference, systemLocale)
	return &Translator{preference: resolved, localizer: i18n.NewLocalizer(bundle, string(resolved))}
}

func (translator *Translator) Preference() Preference { return translator.preference }

func (translator *Translator) Text(id, fallback string, data ...any) string {
	var templateData any
	if len(data) > 0 {
		templateData = data[0]
	}
	// A missing locale can return a usable English fallback alongside an error.
	result, _ := translator.localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{ID: id, Other: fallback},
		TemplateData:   templateData,
	})
	if result == "" {
		return renderFallback(fallback, templateData)
	}
	return result
}

func (translator *Translator) Plural(id, one, other string, count int, data ...any) string {
	var templateData any
	if len(data) > 0 {
		templateData = data[0]
	}
	result, _ := translator.localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{ID: id, One: one, Other: other},
		PluralCount:    count,
		TemplateData:   templateData,
	})
	if result == "" {
		if count == 1 {
			return renderFallback(one, templateData)
		}
		return renderFallback(other, templateData)
	}
	return result
}

func renderFallback(message string, data any) string {
	parsed, err := template.New("fallback").Parse(message)
	if err != nil {
		return message
	}
	var output strings.Builder
	if err := parsed.Execute(&output, data); err == nil {
		return output.String()
	}
	return message
}
