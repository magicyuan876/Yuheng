package embed

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func all() *Registry { return NewRegistry(nil, nil) }

// Acceptance (T2.3): an address outside the allow-list is refused.
func TestEverythingOutsideTheListIsRefused(t *testing.T) {
	r := all()
	for _, raw := range []string{
		"",
		"   ",
		"not a url",
		"https://evil.test/watch?v=abc",
		"https://youtube.com.attacker.test/watch?v=abc",
		"https://evil-youtube.com/watch?v=abc",
		"https://notmiro.com/app/board/abc/",
		// The scheme matters: an http iframe in an https page never renders,
		// so allowing it would only produce embeds that silently fail.
		"http://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"//www.youtube.com/watch?v=dQw4w9WgXcQ",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"file:///etc/passwd",
		// Credentials in the address would be sent to the framed page.
		"https://user:pass@www.youtube.com/watch?v=dQw4w9WgXcQ",
	} {
		_, err := r.Resolve(raw)
		require.ErrorIs(t, err, ErrNotEmbeddable, raw)
	}
}

func TestATrustedHostWithAnUnembeddablePathIsStillRefused(t *testing.T) {
	r := all()
	// The host is right and the path is not something the provider frames.
	// Refusing rather than framing the pasted address is what stops a crafted
	// path on a trusted host from becoming the framed page.
	for _, raw := range []string{
		"https://www.youtube.com/account",
		"https://www.youtube.com/watch?v=../../etc",
		"https://www.figma.com/pricing",
		"https://miro.com/signup",
		"https://www.loom.com/looks/nothing",
		"https://example.feishu.cn/settings",
		"https://www.canva.com/help",
	} {
		_, err := r.Resolve(raw)
		require.ErrorIs(t, err, ErrNotEmbeddable, raw)
	}
}

func TestYouTubeAddressesAllResolveToOneEmbedForm(t *testing.T) {
	r := all()
	const want = "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"
	for _, raw := range []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
		"https://youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
	} {
		got, err := r.Resolve(raw)
		require.NoError(t, err, raw)
		require.Equal(t, "youtube", got.Provider)
		require.Equal(t, want, got.EmbedURL, raw)
		require.Equal(t, raw, got.URL, "the address the author pasted is what is stored")
	}
}

func TestTheEmbedAddressIsDerivedAndNeverThePastedOne(t *testing.T) {
	r := all()
	// A tracking or autoplay parameter on the pasted address must not survive
	// into the frame: the embed address is rebuilt from the identifier alone.
	got, err := r.Resolve("https://www.youtube.com/watch?v=dQw4w9WgXcQ&autoplay=1&list=EVIL")
	require.NoError(t, err)
	require.NotContains(t, got.EmbedURL, "autoplay")
	require.NotContains(t, got.EmbedURL, "EVIL")

	lark, err := r.Resolve("https://acme.feishu.cn/docx/AbCdEf?from=evil&token=secret")
	require.NoError(t, err)
	require.Equal(t, "https://acme.feishu.cn/docx/AbCdEf", lark.EmbedURL)
	require.NotContains(t, lark.EmbedURL, "secret")
}

func TestEachProviderResolvesItsOwnAddresses(t *testing.T) {
	r := all()
	cases := []struct {
		raw      string
		provider string
		embed    string
	}{
		{"https://www.bilibili.com/video/BV1GJ411x7h7", "bilibili",
			"https://player.bilibili.com/player.html?bvid=BV1GJ411x7h7"},
		{"https://www.bilibili.com/video/av170001", "bilibili",
			"https://player.bilibili.com/player.html?aid=170001"},
		{"https://www.loom.com/share/abcdef1234567890", "loom",
			"https://www.loom.com/embed/abcdef1234567890"},
		{"https://miro.com/app/board/uXjVO_abc123=/", "miro",
			"https://miro.com/app/live-embed/uXjVO_abc123=/"},
		{"https://acme.feishu.cn/wiki/AbCdEf", "feishu", "https://acme.feishu.cn/wiki/AbCdEf"},
		{"https://www.canva.com/design/DAF123/view", "canva",
			"https://www.canva.com/design/DAF123/view?embed"},
		{"https://www.canva.com/design/DAF123", "canva",
			"https://www.canva.com/design/DAF123/view?embed"},
	}
	for _, tc := range cases {
		got, err := r.Resolve(tc.raw)
		require.NoError(t, err, tc.raw)
		require.Equal(t, tc.provider, got.Provider, tc.raw)
		require.Equal(t, tc.embed, got.EmbedURL, tc.raw)
	}
}

func TestFigmaIsTheOneProviderThatFramesTheOriginalAddress(t *testing.T) {
	r := all()
	got, err := r.Resolve("https://www.figma.com/design/abc123/Board")
	require.NoError(t, err)
	require.Equal(t, "figma", got.Provider)
	// Figma's documented embed API takes the address as a parameter. It is
	// safe because the host was checked first and the value is escaped into
	// Figma's own page rather than becoming the framed address itself.
	require.True(t, strings.HasPrefix(got.EmbedURL, "https://www.figma.com/embed?"))
	require.Contains(t, got.EmbedURL, "url=https%3A%2F%2Fwww.figma.com%2Fdesign%2Fabc123%2FBoard")
}

func TestAHostIsMatchedExactlyUnlessASuffixWasAskedFor(t *testing.T) {
	r := all()
	// A tenant subdomain is allowed where the provider gives every customer
	// one, and nowhere else.
	_, err := r.Resolve("https://acme.feishu.cn/docx/AbCdEf")
	require.NoError(t, err)
	_, err = r.Resolve("https://acme.evil.feishu.cn.attacker.test/docx/AbCdEf")
	require.ErrorIs(t, err, ErrNotEmbeddable)

	// YouTube is matched exactly, so no subdomain of it is allowed.
	_, err = r.Resolve("https://anything.youtube.com/watch?v=dQw4w9WgXcQ")
	require.ErrorIs(t, err, ErrNotEmbeddable)

	// A trailing dot is the absolute form of the same name and must not slip
	// past the comparison.
	_, err = r.Resolve("https://www.youtube.com./watch?v=dQw4w9WgXcQ")
	require.NoError(t, err)

	// So must an upper-case host resolve the same way.
	got, err := r.Resolve("https://WWW.YouTube.COM/watch?v=dQw4w9WgXcQ")
	require.NoError(t, err)
	require.Equal(t, "youtube", got.Provider)
}

func TestTheListIsConfigurable(t *testing.T) {
	only := NewRegistry([]string{"youtube"}, nil)
	_, err := only.Resolve("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	require.NoError(t, err)
	_, err = only.Resolve("https://www.figma.com/design/abc123/Board")
	require.ErrorIs(t, err, ErrNotEmbeddable, "a provider left off the list is refused")
	require.Equal(t, []string{"youtube"}, only.Names())

	// A typo narrows the list rather than widening it or failing to boot.
	typo := NewRegistry([]string{"youtube", "youtoob"}, nil)
	require.Equal(t, []string{"youtube"}, typo.Names())
}

func TestASelfHostedServiceCanBeAllowedByConfiguration(t *testing.T) {
	r := NewRegistry([]string{"youtube"}, []string{"video.corp.example.com", ".media.corp.example.com"})

	got, err := r.Resolve("https://video.corp.example.com/watch/42?quality=hd")
	require.NoError(t, err)
	require.Equal(t, ProviderExternal, got.Provider)
	// Nothing here knows how to derive an embed address for a service it has
	// never heard of, so the address is framed as given.
	require.Equal(t, "https://video.corp.example.com/watch/42?quality=hd", got.EmbedURL)

	_, err = r.Resolve("https://studio.media.corp.example.com/clip/7")
	require.NoError(t, err, "a leading dot allows the subdomains")

	_, err = r.Resolve("https://other.corp.example.com/watch/42")
	require.ErrorIs(t, err, ErrNotEmbeddable)

	require.Equal(t, []string{"youtube", ProviderExternal}, r.Names())
}

// A node stored before a provider was taken off the list stops being framed,
// rather than continuing to load from a source the deployment has since
// decided against.
func TestAStoredNodeIsRecheckedAgainstTheCurrentPolicy(t *testing.T) {
	before := all()
	got, err := before.Resolve("https://www.figma.com/design/abc123/Board")
	require.NoError(t, err)
	require.True(t, before.Allows(got.Provider, got.URL))
	require.NotEmpty(t, before.EmbedURL(got.Provider, got.URL))

	after := NewRegistry([]string{"youtube"}, nil)
	require.False(t, after.Allows(got.Provider, got.URL))
	require.Empty(t, after.EmbedURL(got.Provider, got.URL))

	// And a node whose provider does not match its address is refused, so a
	// crafted document cannot borrow a trusted provider's name.
	require.False(t, before.Allows("youtube", "https://www.figma.com/design/abc123/Board"))
	require.Empty(t, before.EmbedURL("youtube", "https://www.figma.com/design/abc123/Board"))
}

func TestANilRegistryAllowsNothing(t *testing.T) {
	var r *Registry
	_, err := r.Resolve("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	require.ErrorIs(t, err, ErrNotEmbeddable)
	require.False(t, r.Allows("youtube", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"))
}

func TestAFreshEmbedGetsAShapeToStartFrom(t *testing.T) {
	r := all()
	video, err := r.Resolve("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	require.NoError(t, err)
	require.InDelta(t, 16.0/9.0, video.AspectRatio, 0.001)

	board, err := r.Resolve("https://miro.com/app/board/uXjVO_abc123=/")
	require.NoError(t, err)
	require.InDelta(t, 4.0/3.0, board.AspectRatio, 0.001)
}
