package middleware

import (
	"net/http"
	"regexp"

	"api-gateway/internal/config"
	"api-gateway/pkg/logger"
)

// URLRewriter provides URL rewriting functionality
type URLRewriter struct {
	log logger.Logger
}

// NewURLRewriter creates a new URL rewriting middleware
func NewURLRewriter(log logger.Logger) *URLRewriter {
	return &URLRewriter{
		log: log,
	}
}

// Rewrite applies URL rewriting patterns to requests
func (u *URLRewriter) Rewrite(next http.Handler, rewriteConfig *config.URLRewrite) http.Handler {
	// Compile all regex patterns at initialization
	var patterns []*rewritePattern
	if rewriteConfig != nil {
		for _, p := range rewriteConfig.Patterns {
			regex, err := regexp.Compile(p.Match)
			if err != nil {
				u.log.Error("Failed to compile URL rewrite pattern",
					logger.String("pattern", p.Match),
					logger.Error(err),
				)
				continue
			}
			patterns = append(patterns, &rewritePattern{
				regex:       regex,
				replacement: p.Replacement,
			})
		}
	}

	// If no valid patterns, just pass through
	if len(patterns) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalPath := r.URL.Path

		// Apply each rewrite pattern
		for _, pattern := range patterns {
			if pattern.regex.MatchString(r.URL.Path) {
				newPath := pattern.regex.ReplaceAllString(r.URL.Path, pattern.replacement)
				r.URL.Path = newPath

				u.log.Debug("URL rewritten",
					logger.String("original", originalPath),
					logger.String("rewritten", newPath),
				)

				// We only apply the first matching pattern
				break
			}
		}

		next.ServeHTTP(w, r)
	})
}

// rewritePattern represents a compiled rewrite pattern
type rewritePattern struct {
	regex       *regexp.Regexp
	replacement string
}
