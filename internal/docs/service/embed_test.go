package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/embed"
	"github.com/stretchr/testify/require"
)

// embedBody builds a document framing one external page.
func embedBody(provider, url string) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"type": "doc", "content": []any{
			map[string]any{"type": "embed", "attrs": map[string]any{"provider": provider, "url": url}},
		},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func newEmbedEnv(t *testing.T, providers []string, hosts []string) *pageEnv {
	t.Helper()
	return newPageEnvWith(t, func(d *Deps) {
		d.Embeds = embed.NewRegistry(providers, hosts)
	})
}

// Acceptance (T2.3): an embed of a host outside the allow-list is refused.
//
// It is refused on the way in, not only when the page is rendered, so a
// document never comes to hold an iframe the deployment disallows.
func TestSavingRefusesAnEmbedOutsideTheAllowList(t *testing.T) {
	p := newEmbedEnv(t, nil, nil)
	page := p.create(t, p.alice, nil, "Page")

	for _, tc := range []struct{ provider, url string }{
		{"youtube", "https://evil.test/watch?v=abc"},
		{"external", "https://evil.test/anything"},
		// A trusted provider's name on somebody else's address must not pass.
		{"youtube", "https://www.figma.com/design/abc123/Board"},
		// Nor a trusted host with a path it cannot embed.
		{"youtube", "https://www.youtube.com/account"},
		{"youtube", "http://www.youtube.com/watch?v=dQw4w9WgXcQ"},
	} {
		_, err := p.svc.Collab.Persist(ctx(), PersistInput{
			TenantID: 1, PageID: page.ID, BaseVersion: 0,
			YDoc: []byte("state"), Content: embedBody(tc.provider, tc.url), EditorIDs: []string{"alice"},
		})
		require.Equal(t, 400, httpCode(t, err), tc.url)
	}

	stored, err := p.repos.Pages.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), stored.YDocVersion, "not one of them was stored")
}

func TestSavingAcceptsAnEmbedOnTheAllowList(t *testing.T) {
	p := newEmbedEnv(t, nil, nil)
	page := p.create(t, p.alice, nil, "Page")

	res, err := p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0, YDoc: []byte("state"),
		Content:   embedBody("youtube", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"),
		EditorIDs: []string{"alice"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.YDocVersion)
}

// The same rule applies to a body written by an import, a restore or the REST
// endpoint, because they all go through the one replace path.
func TestReplacingRefusesAnEmbedOutsideTheAllowList(t *testing.T) {
	p := newEmbedEnv(t, nil, nil)
	page := p.create(t, p.alice, nil, "Page")

	_, err := p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID), ReplaceInput{
		Content: embedBody("youtube", "https://evil.test/watch?v=abc"), Reason: ReplaceReasonImport,
	})
	require.Equal(t, 400, httpCode(t, err))

	_, err = p.svc.Pages.ReplaceContent(ctx(), p.alice, p.decision(t, p.alice, page.ID), ReplaceInput{
		Content: embedBody("youtube", "https://youtu.be/dQw4w9WgXcQ"), Reason: ReplaceReasonImport,
	})
	require.NoError(t, err)
}

// A build that forgot to configure the policy embeds nothing, rather than
// everything.
func TestADeploymentWithNoPolicyEmbedsNothing(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "Page")

	_, err := p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0, YDoc: []byte("state"),
		Content:   embedBody("youtube", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"),
		EditorIDs: []string{"alice"},
	})
	require.Equal(t, 400, httpCode(t, err))

	_, err = p.svc.Pages.ResolveEmbed(ctx(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	require.Equal(t, 400, httpCode(t, err))
}

func TestAPageMayNotBeMostlyIframes(t *testing.T) {
	p := newEmbedEnv(t, nil, nil)
	page := p.create(t, p.alice, nil, "Page")

	content := make([]any, 0, MaxEmbedsPerPage+1)
	for i := 0; i <= MaxEmbedsPerPage; i++ {
		content = append(content, map[string]any{"type": "embed", "attrs": map[string]any{
			"provider": "youtube", "url": fmt.Sprintf("https://youtu.be/vid%08d", i),
		}})
	}
	body, err := json.Marshal(map[string]any{"type": "doc", "content": content})
	require.NoError(t, err)

	_, err = p.svc.Collab.Persist(ctx(), PersistInput{
		TenantID: 1, PageID: page.ID, BaseVersion: 0,
		YDoc: []byte("state"), Content: body, EditorIDs: []string{"alice"},
	})
	require.Equal(t, 400, httpCode(t, err))
}

func TestResolvingAnAddressAnswersWhatToFrame(t *testing.T) {
	p := newEmbedEnv(t, []string{"youtube"}, nil)

	view, err := p.svc.Pages.ResolveEmbed(ctx(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=30")
	require.NoError(t, err)
	require.Equal(t, "youtube", view.Provider)
	require.Equal(t, "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", view.EmbedURL)
	require.NotContains(t, view.EmbedURL, "t=30", "the frame gets a derived address, not the pasted one")
	require.InDelta(t, 16.0/9.0, view.AspectRatio, 0.001)

	_, err = p.svc.Pages.ResolveEmbed(ctx(), "https://www.figma.com/design/abc/Board")
	require.Equal(t, 400, httpCode(t, err), "a provider left off this deployment's list is refused")
}

// The title is a nicety. A provider that is slow, broken or lying must not be
// able to turn a working embed into a failure.
func TestAFailedMetadataFetchStillProducesAWorkingEmbed(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	p := newEmbedEnv(t, []string{"youtube"}, nil)
	// Point the client at a server that always fails; the endpoint itself is
	// a constant, so redirecting the transport is how the failure is staged.
	p.svc.Pages.d.HTTPClient = &http.Client{Transport: roundTripTo(broken.URL)}

	view, err := p.svc.Pages.ResolveEmbed(ctx(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	require.NoError(t, err)
	require.Empty(t, view.Title)
	require.NotEmpty(t, view.EmbedURL)
}

func TestAMetadataTitleIsBoundedBeforeItIsUsed(t *testing.T) {
	long := ""
	for i := 0; i < 2000; i++ {
		long += "x"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"title": long, "author_name": long})
	}))
	defer server.Close()

	p := newEmbedEnv(t, []string{"youtube"}, nil)
	p.svc.Pages.d.HTTPClient = &http.Client{Transport: roundTripTo(server.URL)}

	view, err := p.svc.Pages.ResolveEmbed(ctx(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	require.NoError(t, err)
	require.LessOrEqual(t, len([]rune(view.Title)), 300, "a title from somebody else's server is bounded")
	require.LessOrEqual(t, len([]rune(view.Author)), 120)
}

func TestTheEditorIsToldWhatThisDeploymentAllows(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) {
		d.Embeds = embed.NewRegistry([]string{"youtube", "figma"}, []string{"video.corp.example.test"})
		d.DrawioURL = "https://draw.corp.example.test"
	})
	policy := p.svc.Pages.EmbedPolicy()
	require.Equal(t, []string{"youtube", "figma", embed.ProviderExternal}, policy.Providers)
	require.Equal(t, "https://draw.corp.example.test", policy.DrawioURL)

	// With nothing configured the editor is told so, rather than offering
	// something that will be refused on save.
	bare := newPageEnv(t).svc.Pages.EmbedPolicy()
	require.Empty(t, bare.Providers)
	require.Empty(t, bare.DrawioURL)
}

// roundTripTo sends every request to one test server, whatever it was
// addressed to.
func roundTripTo(base string) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		redirected := req.Clone(req.Context())
		parsed, err := http.NewRequest(req.Method, base+req.URL.Path+"?"+req.URL.RawQuery, nil)
		if err != nil {
			return nil, err
		}
		redirected.URL = parsed.URL
		redirected.Host = parsed.Host
		return http.DefaultTransport.RoundTrip(redirected)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
