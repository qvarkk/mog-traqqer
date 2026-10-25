package i18n

import (
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// Localizer translates message IDs into one language. Every method tolerates a
// nil receiver and returns its fallback, so callers never check whether the
// middleware ran.
type Localizer struct {
	localizer *goi18n.Localizer
	language  string
}

func (l *Localizer) Language() string {
	if l == nil {
		return ""
	}
	return l.language
}

// Translate resolves messageID, falling back to the literal English text when no
// catalog has that message. That fallback is what makes translating a codebase
// incremental: ship English literals and add catalog entries one at a time.
func (l *Localizer) Translate(messageID, fallback string, data map[string]any) string {
	if l == nil || l.localizer == nil {
		return fallback
	}

	// DefaultMessage.ID is the message ID and Other is the last resort, after the
	// requested and default languages have both been consulted.
	translated, err := l.localizer.Localize(&goi18n.LocalizeConfig{
		DefaultMessage: &goi18n.Message{ID: messageID, Other: fallback},
		TemplateData:   data,
	})
	if err != nil {
		return fallback
	}

	return translated
}
