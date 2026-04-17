package middleware

import (
	"context"
	"net/http"
	"strings"
)

const defaultLocale = "ja"

// I18n reads Accept-Language header and stores the best-matched locale in context.
// Falls back to the locale stored in JWT claims if present.
func I18n(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := resolveLocale(r)
		// If JWT already set a locale, don't override with header
		if existing, _ := r.Context().Value(ContextKeyLocale).(string); existing != "" {
			locale = existing
		}
		ctx := context.WithValue(r.Context(), ContextKeyLocale, locale)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func resolveLocale(r *http.Request) string {
	accept := r.Header.Get("Accept-Language")
	if accept == "" {
		return defaultLocale
	}
	// Parse tags (simplified)
	for _, part := range strings.Split(accept, ",") {
		tag := strings.TrimSpace(strings.Split(part, ";")[0])
		lang := strings.ToLower(strings.Split(tag, "-")[0])
		switch lang {
		case "ja":
			return "ja"
		case "en":
			return "en"
		}
	}
	return defaultLocale
}
