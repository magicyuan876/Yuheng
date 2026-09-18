package attachment

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// clean sanitises and returns the result as a lower-cased string, which is
// what the assertions below look for scripts in.
func clean(t *testing.T, doc string) string {
	t.Helper()
	out, err := SanitizeSVG([]byte(doc))
	require.NoError(t, err)
	return strings.ToLower(string(out))
}

// Acceptance (T1.6): an uploaded SVG cannot carry script.
func TestSanitizeSVGRemovesEveryWayToRunCode(t *testing.T) {
	cases := map[string]string{
		"script element": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<script>alert(1)</script><circle r="5"/></svg>`,
		"script with CDATA": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<script><![CDATA[alert(1)]]></script><circle r="5"/></svg>`,
		"event handler": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<circle r="5" onload="alert(1)"/></svg>`,
		"click handler": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<circle r="5" onclick="alert(1)"/></svg>`,
		"javascript href": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<a href="javascript:alert(1)"><circle r="5"/></a></svg>`,
		"split javascript href": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<a href="java&#10;script:alert(1)"><circle r="5"/></a></svg>`,
		"style url": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<circle r="5" style="fill:url(javascript:alert(1))"/></svg>`,
		"foreign object": `<svg xmlns="http://www.w3.org/2000/svg">` +
			`<foreignObject><body xmlns="http://www.w3.org/1999/xhtml">` +
			`<script>alert(1)</script></body></foreignObject><circle r="5"/></svg>`,
		"animated href": `<svg xmlns="http://www.w3.org/2000/svg"><a><circle r="5"/>` +
			`<animate attributeName="href" values="javascript:alert(1)"/></a></svg>`,
		"nested script": `<svg xmlns="http://www.w3.org/2000/svg"><g><g>` +
			`<script>alert(1)</script></g><circle r="5"/></g></svg>`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			out := clean(t, doc)
			require.NotContains(t, out, "alert(1)")
			require.NotContains(t, out, "<script")
			require.NotContains(t, out, "javascript:")
			require.NotContains(t, out, "onload")
			require.NotContains(t, out, "onclick")
			require.NotContains(t, out, "foreignobject")
			require.Contains(t, out, "circle", "the drawing itself must survive")
		})
	}
}

func TestSanitizeSVGKeepsTheDrawing(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50" width="100" height="50">` +
		`<defs><linearGradient id="g"><stop offset="0" stop-color="#fff"/></linearGradient></defs>` +
		`<rect x="1" y="2" width="98" height="46" fill="url(#g)" stroke="#333" stroke-width="2"/>` +
		`<text x="10" y="30" font-family="serif" font-size="12">hello</text>` +
		`<use href="#g"/>` +
		`</svg>`
	out := clean(t, doc)
	for _, kept := range []string{
		"viewbox", "lineargradient", "stop-color", "rect", `fill="url(#g)"`,
		"stroke-width", "<text", "hello", `href="#g"`,
	} {
		require.Contains(t, out, kept)
	}
}

func TestSanitizeSVGKeepsUsableReferencesAndDropsTheRest(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg">` +
		`<image href="data:image/png;base64,iVBORw0KGgo="/>` +
		`<image href="https://cdn.example.test/logo.png"/>` +
		`<image href="relative/logo.png"/>` +
		`<image href="data:image/svg+xml;base64,PHN2Zz4="/>` +
		`<image href="file:///etc/passwd"/>` +
		`</svg>`
	out := clean(t, doc)
	require.Contains(t, out, "data:image/png")
	require.Contains(t, out, "https://cdn.example.test/logo.png")
	require.Contains(t, out, "relative/logo.png")
	require.NotContains(t, out, "svg+xml", "a nested SVG document would never be sanitised")
	require.NotContains(t, out, "file://")
}

func TestSanitizeSVGDropsTheDocumentPreamble(t *testing.T) {
	// A DOCTYPE can declare entities and a processing instruction can pull in
	// a stylesheet; neither draws anything, so neither survives.
	doc := `<?xml version="1.0"?>` +
		`<?xml-stylesheet type="text/css" href="https://evil.test/x.css"?>` +
		`<!DOCTYPE svg [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>` +
		`<svg xmlns="http://www.w3.org/2000/svg"><circle r="5"/></svg>`
	out := clean(t, doc)
	require.NotContains(t, out, "stylesheet")
	require.NotContains(t, out, "entity")
	require.NotContains(t, out, "doctype")
	require.Contains(t, out, "circle")
}

func TestSanitizeSVGRefusesWhatIsNotAnSVG(t *testing.T) {
	_, err := SanitizeSVG([]byte(`<html><body>hi</body></html>`))
	require.ErrorIs(t, err, ErrNotSVG)

	_, err = SanitizeSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="5">`))
	require.Error(t, err, "an unbalanced document must not be stored")

	_, err = SanitizeSVG(make([]byte, MaxSVGBytes+1))
	require.Error(t, err)
}

// The sanitiser's output must itself be a valid SVG that survives a second
// pass unchanged; otherwise re-processing a stored file could alter it.
func TestSanitizeSVGIsStable(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
		`<script>alert(1)</script><circle r="5" onload="x()" fill="#f00"/></svg>`
	once, err := SanitizeSVG([]byte(doc))
	require.NoError(t, err)
	twice, err := SanitizeSVG(once)
	require.NoError(t, err)
	require.Equal(t, string(once), string(twice))
	require.True(t, looksLikeSVG(once), "the result must still be recognisable as an SVG")
}
