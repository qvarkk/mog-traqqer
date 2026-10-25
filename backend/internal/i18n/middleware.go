package i18n

import (
	"github.com/gofiber/fiber/v3"
)

// localizerKey is unexported so no other package can collide with it.
type localizerKey struct{}

// Middleware resolves the request language once and stores it for the rest of
// the chain. It must run before the access log, which turns errors into
// responses and therefore needs the language already resolved.
func Middleware(bundle *Bundle) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Priority: query parameter, then cookie, then Accept-Language.
		explicit := c.Query(bundle.queryParam)
		if explicit == "" {
			explicit = c.Cookies(bundle.cookieName)
		}

		localizer := bundle.Negotiate(explicit, c.Get(fiber.HeaderAcceptLanguage))

		c.Locals(localizerKey{}, localizer)
		c.Set(fiber.HeaderContentLanguage, localizer.Language())

		return c.Next()
	}
}

// FromContext returns the localizer for the request, or nil when the middleware
// did not run; the nil-safe Localizer methods then serve English.
func FromContext(c fiber.Ctx) *Localizer {
	localizer, _ := c.Locals(localizerKey{}).(*Localizer)
	return localizer
}
