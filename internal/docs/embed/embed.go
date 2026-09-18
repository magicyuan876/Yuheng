// Package embed decides which external pages a document may put in an iframe,
// and turns the address a person pasted into the one that actually embeds.
//
// An iframe in a document is somebody else's code running next to the reader's
// session. The answer to that is not a clever sanitiser but a short list: a
// closed set of providers, matched on the host and nothing else, with every
// other address refused outright. The list is configurable because a
// self-hosted deployment will have its own video service, and unreachable
// otherwise.
//
// Nothing here performs I/O. Resolving an address is a pure function of the
// address, so the whole of the policy is exercised by its tests; fetching
// metadata over the network is a separate, optional step in the service layer.
package embed

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Provider is one recognised source of embeddable pages.
type Provider struct {
	// Name is what the document stores and the stylesheet keys off.
	Name string
	// Hosts are matched exactly, or as a suffix when they begin with a dot,
	// which is what allows a tenant's own subdomain on a hosted service.
	Hosts []string
	// Embed turns a page address into the address that belongs in the iframe.
	// Returning ok=false means the host is right but the path is not something
	// this provider can embed.
	Embed func(u *url.URL) (string, bool)
	// AspectRatio is the shape the editor gives a fresh embed, as width/height.
	// Zero leaves it to the caller.
	AspectRatio float64
}

// ErrNotEmbeddable is returned for an address no provider claims, or one a
// provider recognises the host of but cannot embed.
var ErrNotEmbeddable = fmt.Errorf("docs: this address cannot be embedded")

// Resolved is a checked, ready-to-embed address.
type Resolved struct {
	// Provider is the name to store on the node.
	Provider string
	// URL is the original address, kept so the node can still link out and so
	// a later change to the embed rules is applied to what the author meant
	// rather than to a derived address.
	URL string
	// EmbedURL is what goes in the iframe. It is derived, never stored.
	EmbedURL string
	// AspectRatio suggests a default height; 0 means no suggestion.
	AspectRatio float64
}

const (
	ratioVideo = 16.0 / 9.0
	ratioBoard = 4.0 / 3.0
)

var (
	reYouTubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{6,32}$`)
	reBilibiliV = regexp.MustCompile(`^(?i)(BV[A-Za-z0-9]{8,12}|av\d{1,15})$`)
	reLoomID    = regexp.MustCompile(`^[A-Za-z0-9]{8,64}$`)
	reMiroID    = regexp.MustCompile(`^[A-Za-z0-9_=-]{8,64}$`)
)

// builtinProviders is the default list. Each one embeds over https only, and
// each derives its iframe address rather than passing the pasted one through,
// so a crafted path on a trusted host cannot become the framed page.
var builtinProviders = []Provider{
	{
		Name:        "youtube",
		Hosts:       []string{"youtube.com", "www.youtube.com", "youtu.be", "www.youtube-nocookie.com", "youtube-nocookie.com"},
		AspectRatio: ratioVideo,
		Embed: func(u *url.URL) (string, bool) {
			id := ""
			switch {
			case strings.EqualFold(u.Hostname(), "youtu.be"):
				id = strings.Trim(u.Path, "/")
			case strings.HasPrefix(u.Path, "/embed/"):
				id = strings.Trim(strings.TrimPrefix(u.Path, "/embed/"), "/")
			case strings.HasPrefix(u.Path, "/shorts/"):
				id = strings.Trim(strings.TrimPrefix(u.Path, "/shorts/"), "/")
			default:
				id = u.Query().Get("v")
			}
			if !reYouTubeID.MatchString(id) {
				return "", false
			}
			// The no-cookie host is the same service without the advertising
			// identifiers, which is the right default inside a workspace.
			return "https://www.youtube-nocookie.com/embed/" + id, true
		},
	},
	{
		Name:        "bilibili",
		Hosts:       []string{"bilibili.com", "www.bilibili.com", "b23.tv", "player.bilibili.com"},
		AspectRatio: ratioVideo,
		Embed: func(u *url.URL) (string, bool) {
			id := ""
			if strings.HasPrefix(u.Path, "/video/") {
				id = strings.Trim(strings.TrimPrefix(u.Path, "/video/"), "/")
				if i := strings.IndexByte(id, '/'); i >= 0 {
					id = id[:i]
				}
			}
			if id == "" {
				if v := u.Query().Get("bvid"); v != "" {
					id = v
				} else if v := u.Query().Get("aid"); v != "" {
					id = "av" + v
				}
			}
			if !reBilibiliV.MatchString(id) {
				return "", false
			}
			if strings.HasPrefix(strings.ToLower(id), "av") {
				return "https://player.bilibili.com/player.html?aid=" + url.QueryEscape(id[2:]), true
			}
			return "https://player.bilibili.com/player.html?bvid=" + url.QueryEscape(id), true
		},
	},
	{
		Name:        "figma",
		Hosts:       []string{"figma.com", "www.figma.com"},
		AspectRatio: ratioBoard,
		Embed: func(u *url.URL) (string, bool) {
			// Figma embeds by handing it the original address, which is the
			// one shape where passing the URL through is the documented API.
			// It is safe because the host was already checked and the value is
			// query-escaped into Figma's own embed page.
			if !strings.HasPrefix(u.Path, "/file/") && !strings.HasPrefix(u.Path, "/design/") &&
				!strings.HasPrefix(u.Path, "/board/") && !strings.HasPrefix(u.Path, "/proto/") &&
				!strings.HasPrefix(u.Path, "/slides/") {
				return "", false
			}
			return "https://www.figma.com/embed?embed_host=yuheng&url=" + url.QueryEscape(u.String()), true
		},
	},
	{
		Name:        "loom",
		Hosts:       []string{"loom.com", "www.loom.com"},
		AspectRatio: ratioVideo,
		Embed: func(u *url.URL) (string, bool) {
			id := ""
			for _, prefix := range []string{"/share/", "/embed/"} {
				if strings.HasPrefix(u.Path, prefix) {
					id = strings.Trim(strings.TrimPrefix(u.Path, prefix), "/")
					break
				}
			}
			if !reLoomID.MatchString(id) {
				return "", false
			}
			return "https://www.loom.com/embed/" + id, true
		},
	},
	{
		Name:        "miro",
		Hosts:       []string{"miro.com", ".miro.com"},
		AspectRatio: ratioBoard,
		Embed: func(u *url.URL) (string, bool) {
			const prefix = "/app/board/"
			if !strings.HasPrefix(u.Path, prefix) {
				return "", false
			}
			id := strings.Trim(strings.TrimPrefix(u.Path, prefix), "/")
			if i := strings.IndexByte(id, '/'); i >= 0 {
				id = id[:i]
			}
			if !reMiroID.MatchString(id) {
				return "", false
			}
			return "https://miro.com/app/live-embed/" + id + "/", true
		},
	},
	{
		Name: "feishu",
		// Every tenant gets its own subdomain, so the suffix form is the only
		// workable match here.
		Hosts:       []string{".feishu.cn", "feishu.cn", ".larksuite.com", "larksuite.com"},
		AspectRatio: ratioBoard,
		Embed: func(u *url.URL) (string, bool) {
			for _, prefix := range []string{"/docx/", "/docs/", "/sheets/", "/base/", "/wiki/", "/file/", "/minutes/"} {
				if strings.HasPrefix(u.Path, prefix) {
					// Lark frames its own document addresses directly; the
					// path is rebuilt rather than passed through so a query
					// string cannot ride along.
					return (&url.URL{Scheme: "https", Host: u.Host, Path: u.Path}).String(), true
				}
			}
			return "", false
		},
	},
	{
		Name:        "canva",
		Hosts:       []string{"canva.com", "www.canva.com"},
		AspectRatio: ratioBoard,
		Embed: func(u *url.URL) (string, bool) {
			if !strings.HasPrefix(u.Path, "/design/") {
				return "", false
			}
			path := strings.TrimRight(u.Path, "/")
			if !strings.HasSuffix(path, "/view") {
				path += "/view"
			}
			return "https://www.canva.com" + path + "?embed", true
		},
	},
}

// Registry is the embed policy of one deployment.
type Registry struct {
	providers []Provider
	// extraHosts are additional hosts allowed under the "external" provider,
	// for a self-hosted video or board service. They are framed as given,
	// because nothing here knows how to derive an embed address for them.
	extraHosts []string
}

// ProviderExternal is the name recorded for a host allowed by configuration
// rather than by a built-in rule.
const ProviderExternal = "external"

// NewRegistry builds the policy.
//
// enabled names the built-in providers to keep; empty means all of them.
// extraHosts adds hosts that are framed as-is, which is how a deployment
// allows its own service without this package having to know anything about
// it. An unknown name in enabled is ignored rather than fatal: a deployment
// should not fail to boot over a typo in an optional list, and the effect of
// ignoring it is a smaller allow-list, never a larger one.
func NewRegistry(enabled []string, extraHosts []string) *Registry {
	r := &Registry{}
	if len(enabled) == 0 {
		r.providers = append(r.providers, builtinProviders...)
	} else {
		wanted := make(map[string]bool, len(enabled))
		for _, name := range enabled {
			wanted[strings.ToLower(strings.TrimSpace(name))] = true
		}
		for _, p := range builtinProviders {
			if wanted[p.Name] {
				r.providers = append(r.providers, p)
			}
		}
	}
	for _, host := range extraHosts {
		if h := normaliseHost(host); h != "" {
			r.extraHosts = append(r.extraHosts, h)
		}
	}
	return r
}

// Names lists the enabled provider names, for the editor to show and for the
// deployment capabilities to report.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.providers)+1)
	for _, p := range r.providers {
		out = append(out, p.Name)
	}
	if len(r.extraHosts) > 0 {
		out = append(out, ProviderExternal)
	}
	return out
}

// Resolve checks an address and returns what to store and what to frame.
//
// Everything is refused by default. An address is embeddable only if it is
// https, has a host on the list, and that host's rule can make an embed
// address out of the path.
func (r *Registry) Resolve(raw string) (*Resolved, error) {
	if r == nil {
		return nil, ErrNotEmbeddable
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrNotEmbeddable
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, ErrNotEmbeddable
	}
	// http is refused as well as everything else: an iframe loaded over plain
	// http in an https page is blocked by the browser anyway, so allowing it
	// would only produce embeds that silently never render.
	if !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil {
		return nil, ErrNotEmbeddable
	}
	host := normaliseHost(u.Hostname())
	if host == "" {
		return nil, ErrNotEmbeddable
	}

	for _, p := range r.providers {
		if !hostMatches(host, p.Hosts) {
			continue
		}
		embedURL, ok := p.Embed(u)
		if !ok {
			return nil, ErrNotEmbeddable
		}
		return &Resolved{
			Provider: p.Name, URL: u.String(), EmbedURL: embedURL, AspectRatio: p.AspectRatio,
		}, nil
	}
	if hostMatches(host, r.extraHosts) {
		return &Resolved{Provider: ProviderExternal, URL: u.String(), EmbedURL: u.String()}, nil
	}
	return nil, ErrNotEmbeddable
}

// Allows reports whether a stored node is still embeddable under the current
// policy. It is checked on every save, so an embed added before a provider was
// removed from the list stops being framed rather than lingering.
func (r *Registry) Allows(provider, rawURL string) bool {
	got, err := r.Resolve(rawURL)
	if err != nil {
		return false
	}
	return got.Provider == provider
}

// EmbedURL derives the address to frame for a stored node, or "" when the node
// is no longer allowed. Renderers call it rather than trusting the stored URL.
func (r *Registry) EmbedURL(provider, rawURL string) string {
	got, err := r.Resolve(rawURL)
	if err != nil || got.Provider != provider {
		return ""
	}
	return got.EmbedURL
}

// hostMatches compares against a list where a leading dot means "and its
// subdomains". An entry without a dot matches that host exactly, so allowing
// "miro.com" never allows "evil-miro.com" or "miro.com.attacker.test".
func hostMatches(host string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, ".") {
			suffix := pattern
			if host == strings.TrimPrefix(pattern, ".") || strings.HasSuffix(host, suffix) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

// normaliseHost lower-cases a host and strips a trailing dot, so the absolute
// form of a name cannot slip past an exact comparison.
func normaliseHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimSuffix(h, ".")
	if strings.ContainsAny(h, "/\\ ") {
		return ""
	}
	return h
}
