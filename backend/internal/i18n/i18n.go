// Package i18n provides per-request translation. Catalogs live in
// locales/*.json and are embedded; the supported languages are discovered from
// those files, so adding one means adding a file. Every lookup carries an
// English fallback, so an untranslated message is served verbatim.
package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/fx"
	"golang.org/x/text/language"

	"github.com/qvarkk/go-fiber-template/backend/internal/config"
)

//go:embed locales/*.json
var localeFiles embed.FS

var Module = fx.Module("i18n", fx.Provide(New))

type Bundle struct {
	bundle    *goi18n.Bundle
	matcher   language.Matcher
	supported []language.Tag
	codes     []string
	// localizers is pre-built per language, so nothing happens per request.
	localizers map[string]*Localizer

	defaultCode string
	queryParam  string
	cookieName  string
}

func New(cfg *config.Config) (*Bundle, error) {
	bundle := goi18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	entries, err := fs.ReadDir(localeFiles, "locales")
	if err != nil {
		return nil, fmt.Errorf("read embedded locales: %w", err)
	}

	var (
		supported []language.Tag
		codes     []string
	)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		file := path.Join("locales", entry.Name())
		if _, err := bundle.LoadMessageFileFS(localeFiles, file); err != nil {
			return nil, fmt.Errorf("load locale %s: %w", file, err)
		}

		code := strings.TrimSuffix(entry.Name(), ".json")
		tag, err := language.Parse(code)
		if err != nil {
			return nil, fmt.Errorf("locale %s is not a valid language tag: %w", file, err)
		}

		supported = append(supported, tag)
		codes = append(codes, tag.String())
	}

	if len(supported) == 0 {
		return nil, errors.New("no locale files found in internal/i18n/locales")
	}

	localizers := make(map[string]*Localizer, len(supported))
	for _, code := range codes {
		localizers[code] = &Localizer{
			localizer: goi18n.NewLocalizer(bundle, code),
			language:  code,
		}
	}

	// A default with no catalog would turn every fallback into a guess, so it
	// is rejected at startup rather than on the first request.
	defaultCode := cfg.I18N.DefaultLanguage
	if _, ok := localizers[defaultCode]; !ok {
		return nil, fmt.Errorf(
			"I18N_DEFAULT_LANGUAGE %q has no catalog; available: %s",
			defaultCode, strings.Join(codes, ", "),
		)
	}

	return &Bundle{
		bundle:      bundle,
		matcher:     language.NewMatcher(supported),
		supported:   supported,
		codes:       codes,
		localizers:  localizers,
		defaultCode: defaultCode,
		queryParam:  cfg.I18N.QueryParam,
		cookieName:  cfg.I18N.CookieName,
	}, nil
}

func (b *Bundle) Codes() []string {
	return append([]string(nil), b.codes...)
}

// Localizer returns the localizer for a preference such as "ru" or
// "ru-RU,ru;q=0.9". The second result is false when nothing matches.
func (b *Bundle) Localizer(preference string) (*Localizer, bool) {
	if preference == "" {
		return nil, false
	}

	tags, _, err := language.ParseAcceptLanguage(preference)
	if err != nil || len(tags) == 0 {
		return nil, false
	}

	_, index, confidence := b.matcher.Match(tags...)
	if confidence == language.No {
		return nil, false
	}

	return b.localizers[b.codes[index]], true
}

func (b *Bundle) Default() *Localizer {
	return b.localizers[b.defaultCode]
}

// Negotiate resolves one request's language: explicit choice, then the
// Accept-Language header, then the default. An unsupported preference falls
// back rather than failing the request.
func (b *Bundle) Negotiate(explicit, acceptLanguage string) *Localizer {
	if localizer, ok := b.Localizer(explicit); ok {
		return localizer
	}
	if localizer, ok := b.Localizer(acceptLanguage); ok {
		return localizer
	}
	return b.Default()
}
