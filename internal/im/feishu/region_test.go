package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/im"
)

// The two regions must stay distinct in every dimension that reaches the
// network or the session store: sending a Lark request to open.feishu.cn fails
// authentication, and reporting the wrong platform merges the two clouds'
// sessions together.
func TestRegions_AreDistinct(t *testing.T) {
	if RegionFeishu.OpenBaseURL == RegionLark.OpenBaseURL {
		t.Errorf("regions share OpenBaseURL %q", RegionFeishu.OpenBaseURL)
	}
	if RegionFeishu.Platform == RegionLark.Platform {
		t.Errorf("regions share Platform %q", RegionFeishu.Platform)
	}
	if RegionFeishu.Platform != im.PlatformFeishu {
		t.Errorf("RegionFeishu.Platform = %q, want %q", RegionFeishu.Platform, im.PlatformFeishu)
	}
	if RegionLark.Platform != im.PlatformLark {
		t.Errorf("RegionLark.Platform = %q, want %q", RegionLark.Platform, im.PlatformLark)
	}
}

func TestRegions_BaseURLHosts(t *testing.T) {
	cases := []struct {
		region Region
		want   string
	}{
		{RegionFeishu, "https://open.feishu.cn"},
		{RegionLark, "https://open.larksuite.com"},
	}
	for _, c := range cases {
		if c.region.OpenBaseURL != c.want {
			t.Errorf("%s OpenBaseURL = %q, want %q", c.region.Label, c.region.OpenBaseURL, c.want)
		}
		if strings.HasSuffix(c.region.OpenBaseURL, "/") {
			t.Errorf("%s OpenBaseURL has a trailing slash: %q", c.region.Label, c.region.OpenBaseURL)
		}
	}
}

// Every API call must land on the region's own cloud.
func TestAdapterAPI_UsesRegionHost(t *testing.T) {
	cases := []struct {
		region     Region
		wantPrefix string
	}{
		{RegionFeishu, "https://open.feishu.cn/open-apis/"},
		{RegionLark, "https://open.larksuite.com/open-apis/"},
	}
	for _, c := range cases {
		a, _ := NewAdapter(c.region, "cli_app", "secret", "", "", "", "", "")

		got := a.api("/open-apis/im/v1/messages/%s/reply", "om_1")
		want := c.wantPrefix + "im/v1/messages/om_1/reply"
		if got != want {
			t.Errorf("%s api() = %q, want %q", c.region.Label, got, want)
		}

		// A path with no format verbs must survive unchanged.
		if got := a.api("/open-apis/cardkit/v1/cards"); got != c.wantPrefix+"cardkit/v1/cards" {
			t.Errorf("%s api() = %q, want %q", c.region.Label, got, c.wantPrefix+"cardkit/v1/cards")
		}
	}
}

func TestAdapterPlatform_FollowsRegion(t *testing.T) {
	aFeishu, _ := NewAdapter(RegionFeishu, "a", "b", "", "", "", "", "")
	if got := aFeishu.Platform(); got != im.PlatformFeishu {
		t.Errorf("Feishu adapter Platform() = %q, want %q", got, im.PlatformFeishu)
	}
	aLark, _ := NewAdapter(RegionLark, "a", "b", "", "", "", "", "")
	if got := aLark.Platform(); got != im.PlatformLark {
		t.Errorf("Lark adapter Platform() = %q, want %q", got, im.PlatformLark)
	}
}

// A custom api_base_url overrides the region's Open Platform host, so internal
// deployments can route Feishu API calls through a reverse proxy.
func TestAdapterAPI_BaseURLOverride(t *testing.T) {
	a := &Adapter{apiBaseURL: "https://feishu-proxy.example.internal"}
	got := a.api("/open-apis/im/v1/messages/%s/reply", "om_1")
	want := "https://feishu-proxy.example.internal/open-apis/im/v1/messages/om_1/reply"
	if got != want {
		t.Errorf("override api() = %q, want %q", got, want)
	}
}

// NewAdapter falls back to the region default when api_base_url is empty, and
// the region default itself passes validation (no SSRF check on the default).
func TestNewAdapter_FallbackAndTrim(t *testing.T) {
	a, err := NewAdapter(RegionFeishu, "a", "b", "", "", "", "", "")
	if err != nil {
		t.Fatalf("NewAdapter empty api_base_url: %v", err)
	}
	if a.apiBaseURL != RegionFeishu.OpenBaseURL {
		t.Errorf("fallback apiBaseURL = %q, want %q", a.apiBaseURL, RegionFeishu.OpenBaseURL)
	}
	// The region default value also passes (treated as "no override").
	a2, err := NewAdapter(RegionFeishu, "a", "b", "", "", RegionFeishu.OpenBaseURL, "", "")
	if err != nil {
		t.Fatalf("NewAdapter default api_base_url: %v", err)
	}
	if a2.apiBaseURL != RegionFeishu.OpenBaseURL {
		t.Errorf("default apiBaseURL = %q", a2.apiBaseURL)
	}
}

// validateAPIBaseURL rejects non-http(s) schemes; empty and the region default
// are allowed without further checks.
func TestValidateAPIBaseURL(t *testing.T) {
	defaults := RegionFeishu.OpenBaseURL
	if err := validateAPIBaseURL("", defaults); err != nil {
		t.Errorf("empty should be allowed: %v", err)
	}
	if err := validateAPIBaseURL(defaults, defaults); err != nil {
		t.Errorf("default should be allowed: %v", err)
	}
	if err := validateAPIBaseURL("ftp://example.com", defaults); err == nil {
		t.Error("ftp scheme should be rejected")
	}
	if err := validateAPIBaseURL("gopher://example.com", defaults); err == nil {
		t.Error("gopher scheme should be rejected")
	}
}

// The streaming card placeholder follows the region so Lark users are not shown
// Chinese copy. The region default is applied by NewAdapter when the channel
// sets no thinking_text.
func TestBuildStreamingCardJSON_PlaceholderFollowsRegion(t *testing.T) {
	for _, region := range []Region{RegionFeishu, RegionLark} {
		a, err := NewAdapter(region, "a", "b", "", "", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		raw := buildStreamingCardJSON(a.cardTitle, a.thinkingText)

		var card struct {
			Config struct {
				StreamingMode bool `json:"streaming_mode"`
				Summary       struct {
					Content string `json:"content"`
				} `json:"summary"`
			} `json:"config"`
			Body struct {
				Elements []struct {
					Content   string `json:"content"`
					ElementID string `json:"element_id"`
				} `json:"elements"`
			} `json:"body"`
		}
		if err := json.Unmarshal([]byte(raw), &card); err != nil {
			t.Fatalf("%s card is not valid JSON: %v", region.Label, err)
		}

		if !card.Config.StreamingMode {
			t.Errorf("%s card has streaming_mode disabled", region.Label)
		}
		if card.Config.Summary.Content != region.ThinkingText {
			t.Errorf("%s summary = %q, want %q", region.Label, card.Config.Summary.Content, region.ThinkingText)
		}
		if len(card.Body.Elements) != 1 {
			t.Fatalf("%s card has %d elements, want 1", region.Label, len(card.Body.Elements))
		}
		el := card.Body.Elements[0]
		if el.ElementID != streamingElementID {
			t.Errorf("%s element_id = %q, want %q", region.Label, el.ElementID, streamingElementID)
		}
		if !strings.Contains(el.Content, region.ThinkingText) {
			t.Errorf("%s element content %q does not contain %q", region.Label, el.Content, region.ThinkingText)
		}
	}

	// Guard the point of the whole exercise: the copy actually differs.
	if RegionFeishu.ThinkingText == RegionLark.ThinkingText {
		t.Error("Feishu and Lark share ThinkingText; Lark users would see Chinese copy")
	}
}

// The streaming card header shows the channel's display name; an unnamed
// channel falls back to the product name.
func TestStreamingCardTitleFollowsChannelName(t *testing.T) {
	titleOf := func(raw string) string {
		var card struct {
			Header struct {
				Title struct {
					Content string `json:"content"`
				} `json:"title"`
			} `json:"header"`
		}
		if err := json.Unmarshal([]byte(raw), &card); err != nil {
			t.Fatalf("card is not valid JSON: %v", err)
		}
		return card.Header.Title.Content
	}

	got := titleOf(buildStreamingCardJSON("Strategic Assistant", RegionFeishu.ThinkingText))
	if got != "Strategic Assistant" {
		t.Errorf("title = %q, want channel name", got)
	}

	named, err := NewAdapter(RegionFeishu, "a", "b", "", "", "", "  Strategic Assistant  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if named.cardTitle != "Strategic Assistant" {
		t.Errorf("cardTitle = %q, want trimmed channel name", named.cardTitle)
	}

	unnamed, err := NewAdapter(RegionFeishu, "a", "b", "", "", "", "  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if unnamed.cardTitle != defaultCardTitle {
		t.Errorf("cardTitle = %q, want fallback %q", unnamed.cardTitle, defaultCardTitle)
	}
}

// The streaming placeholder copy is per-channel configurable via the optional
// thinking_text credential; blank values fall back to the region default.
func TestStreamingThinkingTextConfigurable(t *testing.T) {
	custom, err := NewAdapter(RegionFeishu, "a", "b", "", "", "", "", "  马上就好，正在查资料…  ")
	if err != nil {
		t.Fatal(err)
	}
	if custom.thinkingText != "马上就好，正在查资料…" {
		t.Errorf("thinkingText = %q, want trimmed custom copy", custom.thinkingText)
	}
	raw := buildStreamingCardJSON(custom.cardTitle, custom.thinkingText)
	var card struct {
		Config struct {
			Summary struct {
				Content string `json:"content"`
			} `json:"summary"`
		} `json:"config"`
		Body struct {
			Elements []struct {
				Content string `json:"content"`
			} `json:"elements"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatalf("card is not valid JSON: %v", err)
	}
	if card.Config.Summary.Content != custom.thinkingText {
		t.Errorf("summary = %q, want custom copy", card.Config.Summary.Content)
	}
	if len(card.Body.Elements) != 1 || !strings.Contains(card.Body.Elements[0].Content, custom.thinkingText) {
		t.Errorf("element content %+v does not carry custom copy", card.Body.Elements)
	}

	blank, err := NewAdapter(RegionFeishu, "a", "b", "", "", "", "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if blank.thinkingText != RegionFeishu.ThinkingText {
		t.Errorf("thinkingText = %q, want region fallback %q", blank.thinkingText, RegionFeishu.ThinkingText)
	}
}
