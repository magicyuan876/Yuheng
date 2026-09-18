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
	// CollabInternalBaseURL is the HTTP address this server calls the
	// collaboration service on (replace, evict). It is separate from
	// CollabURL because the browser may reach the service through an
	// ingress ("wss://docs.example.com/collab") while this server talks to
	// it directly ("http://collab:1234"). Empty derives it from CollabURL.
	CollabInternalBaseURL string `yaml:"collab_internal_url" json:"collab_internal_url"`
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
	// EmbedProviders names the built-in embed providers to allow. Empty
	// allows all of them; listing a subset is how a deployment narrows what a
	// document may put in an iframe.
	EmbedProviders []string `yaml:"embed_providers" json:"embed_providers"`
	// EmbedExtraHosts allows additional hosts, framed as given, for a
	// self-hosted video or whiteboard service. A leading dot allows the
	// host's subdomains.
	EmbedExtraHosts []string `yaml:"embed_extra_hosts" json:"embed_extra_hosts"`
	// DrawioURL is the self-hosted draw.io editor the diagram node opens.
	// Empty disables creating and editing draw.io diagrams; existing ones
	// still render from their stored preview.
	DrawioURL string `yaml:"drawio_url" json:"drawio_url"`
}

// DrawioEnabled reports whether a draw.io editor is configured.
func (d *DocsConfig) DrawioEnabled() bool { return d != nil && strings.TrimSpace(d.DrawioURL) != "" }

// CollabEnabled reports whether a collaboration service is configured.
func (d *DocsConfig) CollabEnabled() bool { return d != nil && strings.TrimSpace(d.CollabURL) != "" }

// CollabInternalURL is the HTTP base URL for this server's calls to the
// collaboration service. It prefers the explicit setting and otherwise
// derives one from the browser-facing WebSocket URL (ws → http, wss → https),
// which is right for the Compose default where both are the same host.
func (d *DocsConfig) CollabInternalURL() string {
	if d == nil {
		return ""
	}
	if v := strings.TrimSpace(d.CollabInternalBaseURL); v != "" {
		return strings.TrimRight(v, "/")
	}
	ws := strings.TrimSpace(d.CollabURL)
	switch {
	case ws == "":
		return ""
	case strings.HasPrefix(ws, "wss://"):
		return strings.TrimRight("https://"+strings.TrimPrefix(ws, "wss://"), "/")
	case strings.HasPrefix(ws, "ws://"):
		return strings.TrimRight("http://"+strings.TrimPrefix(ws, "ws://"), "/")
	default:
		return strings.TrimRight(ws, "/")
	}
}

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
		CollabInternalBaseURL:   strings.TrimSpace(os.Getenv("YUHENG_COLLAB_INTERNAL_URL")),
		MaxYDocBytes:            envInt64("YUHENG_DOCS_MAX_YDOC_BYTES", 20*1024*1024),
		MaxAttachmentBytes:      envInt64("YUHENG_DOCS_MAX_ATTACHMENT_BYTES", 200*1024*1024),
		TrashRetentionDays:      int(envInt64("YUHENG_DOCS_TRASH_RETENTION_DAYS", 30)),
		RevisionIntervalMinutes: int(envInt64("YUHENG_DOCS_REVISION_INTERVAL_MINUTES", 10)),
		IndexDebounceSeconds:    int(envInt64("YUHENG_DOCS_INDEX_DEBOUNCE_SECONDS", 60)),
		ACLCacheTTLSeconds:      int(envInt64("YUHENG_DOCS_ACL_CACHE_TTL_SECONDS", 60)),
		EmbedProviders:          envList("YUHENG_DOCS_EMBED_PROVIDERS"),
		EmbedExtraHosts:         envList("YUHENG_DOCS_EMBED_EXTRA_HOSTS"),
		DrawioURL:               strings.TrimSpace(os.Getenv("YUHENG_DOCS_DRAWIO_URL")),
	}
	if d.Enabled && d.CollabEnabled() && d.CollabSharedSecret == "" {
		// Printf: LoadConfig runs before the logger is wired.
		fmt.Println("[config] WARNING: YUHENG_COLLAB_URL is set but YUHENG_COLLAB_SHARED_SECRET is empty; " +
			"the collaboration service will not be able to authenticate its callbacks")
	}
	return d
}

// envList reads a comma-separated setting, dropping blanks. An unset variable
// and one set to an empty string mean the same thing: no explicit list.
func envList(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
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
