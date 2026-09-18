package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/embed"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// Embedding an external page.
//
// The rule is that a document can only ever come to hold an iframe the
// deployment allows. That is enforced on the way in, on the save path both
// editing transports share, rather than only when the page is rendered:
// checking at render time alone would mean a refused embed still sat in the
// stored document, ready to be framed the moment somebody widened the policy
// or wrote a second renderer.

// MaxEmbedsPerPage bounds how many external pages one document may frame.
// Each is a separate origin loading in the reader's browser; a page with
// dozens of them is not a document.
const MaxEmbedsPerPage = 40

// checkEmbeds refuses a document that frames anything the policy does not
// allow. It returns a validation error naming the address, because the person
// who pasted it is the one who can fix it.
func (b *base) checkEmbeds(st render.Structure) error {
	if len(st.Embeds) == 0 {
		return nil
	}
	if len(st.Embeds) > MaxEmbedsPerPage {
		return invalid("a page may embed at most %d external pages", MaxEmbedsPerPage)
	}
	// A deployment with no registry configured embeds nothing. That is the
	// safe direction: the alternative is a build that silently frames
	// anything.
	if b.d.Embeds == nil {
		return invalid("this deployment does not allow embedded pages")
	}
	for _, e := range st.Embeds {
		if !b.d.Embeds.Allows(e.Provider, e.URL) {
			return invalid("%q cannot be embedded here", truncateURL(e.URL))
		}
	}
	return nil
}

// truncateURL keeps an error message readable, and keeps a very long pasted
// address out of the logs it ends up in.
func truncateURL(raw string) string {
	const max = 120
	if len([]rune(raw)) <= max {
		return raw
	}
	return string([]rune(raw)[:max]) + "…"
}

// EmbedView is what the editor is told about an address somebody pasted.
type EmbedView struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
	// EmbedURL is what the client frames. It is derived on every read rather
	// than stored, so a change to the policy or to a provider's embed form
	// applies to existing documents without rewriting them.
	EmbedURL string `json:"embed_url"`
	// Title and Author come from the provider's own oEmbed endpoint when it
	// has one and it answered; both are optional and purely cosmetic.
	Title  string `json:"title,omitempty"`
	Author string `json:"author,omitempty"`
	// AspectRatio suggests a height for a fresh embed; 0 means no suggestion.
	AspectRatio float64 `json:"aspect_ratio,omitempty"`
}

// oEmbedEndpoints are the providers that publish one. A provider absent from
// this map simply has no title; the embed works either way, which is why this
// is a best-effort lookup and never a gate.
var oEmbedEndpoints = map[string]string{
	"youtube":  "https://www.youtube.com/oembed",
	"bilibili": "",
	"loom":     "https://www.loom.com/v1/oembed",
	"figma":    "https://www.figma.com/api/oembed",
	"canva":    "",
	"miro":     "",
	"feishu":   "",
}

// oEmbedTimeout bounds the metadata fetch. It is a nicety on an interactive
// path, so it gives up quickly rather than making somebody wait.
const oEmbedTimeout = 4 * time.Second

// maxOEmbedBytes bounds the response read into memory.
const maxOEmbedBytes = 64 << 10

// ResolveEmbed checks an address against the allow-list and, when the provider
// publishes one, fetches its title.
//
// The check is what matters and happens first; the fetch is optional and its
// failure is invisible. The fetch is also the only place this package makes an
// outbound request on a user-supplied address, so it goes through the same
// SSRF validation as every other such request in the server, and only ever to
// the provider's own endpoint rather than to the pasted address.
func (s *PageService) ResolveEmbed(ctx context.Context, raw string) (*EmbedView, error) {
	if s.d.Embeds == nil {
		return nil, invalid("this deployment does not allow embedded pages")
	}
	resolved, err := s.d.Embeds.Resolve(raw)
	if err != nil {
		return nil, invalid("%q cannot be embedded here", truncateURL(strings.TrimSpace(raw)))
	}
	out := &EmbedView{
		Provider: resolved.Provider, URL: resolved.URL,
		EmbedURL: resolved.EmbedURL, AspectRatio: resolved.AspectRatio,
	}
	if title, author, ok := s.fetchOEmbed(ctx, resolved.Provider, resolved.URL); ok {
		out.Title, out.Author = title, author
	}
	return out, nil
}

// fetchOEmbed asks the provider what the page is called. Every failure is
// silent: a missing title costs nothing, and an error here would turn a
// working embed into a refusal.
func (s *PageService) fetchOEmbed(ctx context.Context, provider, pageURL string) (string, string, bool) {
	endpoint := oEmbedEndpoints[provider]
	if endpoint == "" {
		return "", "", false
	}
	// The endpoint is one of this package's own constants and the pasted
	// address only ever travels inside a query parameter, but the validation
	// runs anyway: it is the rule for every outbound request in this server,
	// and an exception is how one eventually gets missed.
	if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
		return "", "", false
	}
	target := endpoint + "?format=json&url=" + url.QueryEscape(pageURL)

	fetchCtx, cancel := context.WithTimeout(ctx, oEmbedTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, target, nil)
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Accept", "application/json")

	client := s.d.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: oEmbedTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOEmbedBytes))
	if err != nil {
		return "", "", false
	}
	var payload struct {
		Title      string `json:"title"`
		AuthorName string `json:"author_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", false
	}
	return clip(payload.Title, 300), clip(payload.AuthorName, 120), payload.Title != ""
}

// clip bounds a string from somebody else's server before it is stored or
// shown.
func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// EmbedPolicy is what the editor needs to know about this deployment's rules.
type EmbedPolicy struct {
	// Providers are the names an editor may offer.
	Providers []string `json:"providers"`
	// DrawioURL is the self-hosted draw.io editor, or empty when diagrams
	// cannot be created or edited here.
	DrawioURL string `json:"drawio_url,omitempty"`
}

// EmbedPolicy reports the deployment's embedding rules.
func (s *PageService) EmbedPolicy() *EmbedPolicy {
	out := &EmbedPolicy{Providers: []string{}, DrawioURL: s.d.DrawioURL}
	if s.d.Embeds != nil {
		out.Providers = s.d.Embeds.Names()
	}
	return out
}

// Embeds is the embed policy as this package uses it. embed.Registry
// satisfies it; an interface keeps the service testable without one.
type Embeds interface {
	Resolve(raw string) (*embed.Resolved, error)
	Allows(provider, rawURL string) bool
	EmbedURL(provider, rawURL string) string
	Names() []string
}

var _ Embeds = (*embed.Registry)(nil)
