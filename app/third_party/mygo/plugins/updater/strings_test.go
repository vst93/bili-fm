package updater

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestMatchLanguage(t *testing.T) {
	available := []string{"de", "en", "pt-BR", "sv", "zh-Hans", "zh-Hant"}
	for locale, want := range map[string]string{
		"de-DE":       "de",
		"de_AT.UTF-8": "de",
		"en-GB":       "en",
		"pt-BR":       "pt-BR",
		"pt-PT":       "pt-BR",
		"zh-Hans-CN":  "zh-Hans",
		"zh-CN":       "zh-Hans",
		"zh_SG":       "zh-Hans",
		"zh":          "zh-Hans",
		"zh-Hant-HK":  "zh-Hant",
		"zh-TW":       "zh-Hant",
		"zh_HK.UTF-8": "zh-Hant",
		"zh-MO":       "zh-Hant",
		"SV_se":       "sv",
		"es-419":      "en",
		"sr-Latn-RS":  "en",
		"":            "en",
		"C":           "en",
	} {
		if got := matchLanguage(locale, available); got != want {
			t.Errorf("matchLanguage(%q) = %q, want %q", locale, got, want)
		}
	}
	if got := matchLanguage("zh-TW", []string{"en", "zh-Hans"}); got != "en" {
		t.Errorf("zh-TW got %q: one script of Chinese is no fallback for the other", got)
	}
}

// The plugin's translations have every text, and their formats use every
// argument.
func TestTranslations(t *testing.T) {
	all := map[string]Strings{"en": english}
	for l, s := range translations {
		all[l] = s
	}
	for lang, s := range all {
		if err := s.check(); err != nil {
			t.Errorf("%s: %v", lang, err)
		}
		v := reflect.ValueOf(s)
		for i := range v.NumField() {
			name, text := v.Type().Field(i).Name, v.Field(i).String()
			n, format := formats[name]
			switch {
			case text == "":
				t.Errorf("%s: no %s", lang, name)
			case !format && strings.Contains(text, "%"):
				t.Errorf("%s: %s is not a format: %q", lang, name, text)
			case format:
				args := make([]any, n)
				for j := range args {
					args[j] = "<" + strconv.Itoa(j) + ">"
				}
				out := fmt.Sprintf(text, args...)
				for _, a := range args {
					if !strings.Contains(out, a.(string)) {
						t.Errorf("%s: %s %q leaves out argument %s", lang, name, text, a)
					}
				}
			}
		}
	}
}

func TestText(t *testing.T) {
	de := newText("de-AT", nil)
	if de.lang != "de" || de.Install != "Update installieren" || de.megabytes(12_345_678) != "12,3 MB" {
		t.Errorf("de: %q, %q, %q", de.lang, de.Install, de.megabytes(12_345_678))
	}
	if fr := newText("fr-CA", nil); fr.megabytes(1_500_000) != "1,5 Mo" {
		t.Errorf("fr: %q", fr.megabytes(1_500_000))
	}
	if ja := newText("ja-JP", nil); ja.megabytes(1_500_000) != "1.5 MB" {
		t.Errorf("ja: %q", ja.megabytes(1_500_000))
	}

	extra := map[string]Strings{
		"EN": {Install: "Update Now"},
		"ar": {Title: "تحديث البرامج"},
		"sv": {Install: "Installera uppdatering"},
	}
	en := newText("en-US", extra)
	if en.Install != "Update Now" || en.Skip != "Skip This Version" {
		t.Errorf("en: %q, %q", en.Install, en.Skip)
	}
	sv := newText("sv-SE", extra)
	if sv.lang != "sv" || sv.Install != "Installera uppdatering" || sv.Skip != "Skip This Version" {
		t.Errorf("sv: %q, %q, %q", sv.lang, sv.Install, sv.Skip)
	}
	if ar := newText("ar-EG", extra); !ar.rtl || ar.Title != "تحديث البرامج" {
		t.Errorf("ar: rtl %v, %q", ar.rtl, ar.Title)
	}
	if de := newText("de", extra); de.Install != "Update installieren" {
		t.Errorf("another language's strings leaked into de: %q", de.Install)
	}
}

func TestInvalidStrings(t *testing.T) {
	p := New(Options{Strings: map[string]Strings{"en": {Available: "A new version of %d"}}})
	if err := p.Setup(); err == nil || !strings.Contains(err.Error(), "Available") {
		t.Errorf("Setup: %v", err)
	}
	if active.Load() != nil {
		t.Error("an invalid plugin became the active one")
	}
}
