package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DocsConfig configures the online documents module. Every value comes from
// the YUHENG_DOCS_* / YUHENG_COLLAB_* environment variables documented in
// .env.example (group K); the module is off unless YUHENG_DOCS_ENABLED=true.
type DocsConfig struct {
	// Enabled registers the /api/v1/docs routes and starts the module.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// CollabURL is the browser-facing WebSocket URL of the collaboration
	// service (ws://collab:1234 in Compose). Empty means no collaboration
	// service: pages use exclusive-edit leases instead (Lite edition).
	CollabURL string `yaml:"collab_url" json:"collab_url"`
	// CollabSharedSecret signs the HTTP callbacks between the collaboration
	// service and this server. Required whenever CollabURL is set.
	CollabSharedSecret string `yaml:"collab_shared_secret" json:"-"`
	// MaxYDocBytes caps the Yjs state of one page (default 20 MiB).
	MaxYDocBytes int64 `yaml:"max_ydoc_bytes" json:"max_ydoc_bytes"`
	// MaxAttachmentBytes caps one uploaded file (default 200 MiB).
	MaxAttachmentBytes int64 `yaml:"max_attachment_bytes" json:"max_attachment_bytes"`
	// TrashRetentionDays is how long deleted pages stay restorable (default 30).
	TrashRetentionDays int `yaml:"trash_retention_days" json:"trash_retention_days"`
	// RevisionIntervalMinutes is the minimum spacing between automatic
	// history snapshots of one page (default 10).
	RevisionIntervalMinutes int `yaml:"revision_interval_minutes" json:"revision_interval_minutes"`
	// IndexDebounceSeconds delays knowledge-base re-indexing after an edit
	// so a burst of saves becomes one rebuild (default 60).
	IndexDebounceSeconds int `yaml:"index_debounce_seconds" json:"index_debounce_seconds"`
	// ACLCacheTTLSeconds bounds how long a permission decision may be served
	// from cache (default 60).
	ACLCacheTTLSeconds int `yaml:"acl_cache_ttl_seconds" json:"acl_cache_ttl_seconds"`
}

// CollabEnabled reports whether a collaboration service is configured.
func (d *DocsConfig) CollabEnabled() bool { return d != nil && strings.TrimSpace(d.CollabURL) != "" }

// IsEnabled is nil-safe.
func (d *DocsConfig) IsEnabled() bool { return d != nil && d.Enabled }

// loadDocsConfig reads the module's environment. Defaults are the values the
// design document specifies; malformed numbers fall back to the default with
// a startup warning rather than aborting boot.
func loadDocsConfig() *DocsConfig {
	d := &DocsConfig{
		Enabled:                 envBool("YUHENG_DOCS_ENABLED", false),
		CollabURL:               strings.TrimSpace(os.Getenv("YUHENG_COLLAB_URL")),
		CollabSharedSecret:      strings.TrimSpace(os.Getenv("YUHENG_COLLAB_SHARED_SECRET")),
		MaxYDocBytes:            envInt64("YUHENG_DOCS_MAX_YDOC_BYTES", 20*1024*1024),
		MaxAttachmentBytes:      envInt64("YUHENG_DOCS_MAX_ATTACHMENT_BYTES", 200*1024*1024),
		TrashRetentionDays:      int(envInt64("YUHENG_DOCS_TRASH_RETENTION_DAYS", 30)),
		RevisionIntervalMinutes: int(envInt64("YUHENG_DOCS_REVISION_INTERVAL_MINUTES", 10)),
		IndexDebounceSeconds:    int(envInt64("YUHENG_DOCS_INDEX_DEBOUNCE_SECONDS", 60)),
		ACLCacheTTLSeconds:      int(envInt64("YUHENG_DOCS_ACL_CACHE_TTL_SECONDS", 60)),
	}
	if d.Enabled && d.CollabEnabled() && d.CollabSharedSecret == "" {
		// Printf: LoadConfig runs before the logger is wired.
		fmt.Println("[config] WARNING: YUHENG_COLLAB_URL is set but YUHENG_COLLAB_SHARED_SECRET is empty; " +
			"the collaboration service will not be able to authenticate its callbacks")
	}
	return d
}

func envBool(name string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	fmt.Printf("[config] WARNING: %s=%q is not a boolean; using %v\n", name, v, def)
	return def
}

func envInt64(name string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		fmt.Printf("[config] WARNING: %s=%q is not a non-negative integer; using %d\n", name, v, def)
		return def
	}
	return n
}
