// Package router registers the HTTP routes and the middleware around them.
package router

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
)

// corsConfig builds the CORS policy from YUHENG_CORS_ALLOWED_ORIGINS, a
// comma-separated list of origins (https://app.example.com). Unset, empty or
// containing "*" means any origin.
//
// Credentials are advertised only for an explicit list. Yuheng authenticates
// with Authorization / X-API-Key headers that a script attaches itself, never
// with ambient cookies, so no request needs browser credentials; and a
// wildcard Access-Control-Allow-Origin combined with
// Access-Control-Allow-Credentials: true is a pair the CORS specification
// forbids and browsers reject. Sending only the half that can work keeps the
// response honest. gin-contrib/cors turns "*" into AllowAllOrigins and would
// otherwise emit exactly that contradictory pair.
func corsConfig() cors.Config {
	origins, credentials := corsOrigins(os.Getenv("YUHENG_CORS_ALLOWED_ORIGINS"))
	return cors.Config{
		AllowOrigins: origins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key", "X-Request-ID", "X-Tenant-ID",
			"X-Embed-Session", "X-External-User-ID", "X-External-User-Token",
		},
		ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials: credentials,
		MaxAge:           12 * time.Hour,
	}
}

// corsOrigins parses the allowed-origins setting and reports whether
// credentials may accompany it (only for an explicit list).
func corsOrigins(raw string) (origins []string, credentials bool) {
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			return []string{"*"}, false
		}
		origins = append(origins, o)
	}
	if len(origins) == 0 {
		return []string{"*"}, false
	}
	return origins, true
}
