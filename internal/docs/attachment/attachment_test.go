package attachment

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// A gradient, so a resize that dropped pixels instead of
			// averaging them would show up as a different colour.
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: uint8(x % 256), B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// mp4Header is the smallest ISO base-media header a sniffer accepts: a
// 32-byte ftyp box whose brand list contains mp41.
func mp4Header() []byte {
	head := []byte{0, 0, 0, 0x20}
	head = append(head, "ftypisom"...)
	head = append(head, 0, 0, 2, 0)
	return append(head, "isomiso2avc1mp41"...)
}

func TestSniffTrustsTheBytesNotTheName(t *testing.T) {
	png16 := pngBytes(t, 16, 16)

	// A PNG called .txt is still a PNG.
	got := Sniff(png16, "notes.txt")
	require.Equal(t, "image/png", got.Media)
	require.Equal(t, KindImage, got.Kind)
	require.Equal(t, ".png", got.Ext)

	// And a text file called .png is still text, which is the case that
	// matters: it must not be served as an image.
	got = Sniff([]byte("just some words\n"), "avatar.png")
	require.Equal(t, "text/plain", got.Media)
	require.Equal(t, KindFile, got.Kind)
}

func TestSniffRecognisesTheCommonMediaTypes(t *testing.T) {
	cases := []struct {
		name  string
		head  []byte
		file  string
		media string
		kind  Kind
	}{
		{"png", pngBytes(t, 4, 4), "a.png", "image/png", KindImage},
		{"jpeg", jpegBytes(t, 4, 4), "a.jpg", "image/jpeg", KindImage},
		{"gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), "a.gif", "image/gif", KindImage},
		{"pdf", []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"), "a.pdf", MediaPDF, KindFile},
		{"mp4", mp4Header(), "a.mp4", "video/mp4", KindVideo},
		{"mp3", []byte("ID3\x03\x00\x00\x00\x00\x00\x00"), "a.mp3", "audio/mpeg", KindAudio},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sniff(tc.head, tc.file)
			require.Equal(t, tc.media, got.Media)
			require.Equal(t, tc.kind, got.Kind)
		})
	}
}

func TestSniffTellsTheOfficeFormatsApartInsideTheZipFamily(t *testing.T) {
	// Every one of them is a zip; only the name distinguishes them, and doing
	// so is safe because none is ever served inline.
	zipHead := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")
	require.Equal(t,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Sniff(zipHead, "report.docx").Media)
	require.Equal(t,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Sniff(zipHead, "budget.xlsx").Media)
	require.Equal(t, "application/zip", Sniff(zipHead, "bundle.zip").Media)
	// The refinement never turns a zip into something renderable.
	require.False(t, ServingFor(Sniff(zipHead, "report.docx").Media).Inline)
}

func TestSniffFindsAnSVGBehindAnyPreamble(t *testing.T) {
	for _, doc := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"\xef\xbb\xbf<svg xmlns=\"http://www.w3.org/2000/svg\"/>",
		"<?xml version=\"1.0\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"<!-- drawn by hand -->\n<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"x.dtd\">\n<svg xmlns=\"http://www.w3.org/2000/svg\"/>",
	} {
		require.Equal(t, MediaSVG, Sniff([]byte(doc), "drawing.svg").Media, doc)
	}
	// An HTML document named .svg is not an SVG and must not be treated as one.
	require.NotEqual(t, MediaSVG, Sniff([]byte("<html><body>hi</body></html>"), "drawing.svg").Media)
	require.NotEqual(t, MediaSVG, Sniff([]byte("<svgx>not it</svgx>"), "drawing.svg").Media)
}

func TestServingRefusesToRenderAnythingItDoesNotRecognise(t *testing.T) {
	// The rule that matters: an uploaded page or script is a download, never
	// a same-origin document.
	for _, media := range []string{
		"text/html", "application/xhtml+xml", "text/javascript", "application/javascript",
		"text/xml", "application/xml", "text/css", "application/octet-stream", "text/plain",
	} {
		s := ServingFor(media)
		require.False(t, s.Inline, media)
		require.Equal(t, MediaOctetStream, s.ContentType, media)
	}

	for _, media := range []string{"image/png", "image/jpeg", "image/gif", "image/webp", "video/mp4"} {
		s := ServingFor(media)
		require.True(t, s.Inline, media)
		require.Equal(t, media, s.ContentType, media)
	}

	// SVG and PDF render, but only inside a sandbox.
	require.True(t, ServingFor(MediaSVG).Inline)
	require.True(t, ServingFor(MediaSVG).Sandbox)
	require.True(t, ServingFor(MediaPDF).Sandbox)
	require.False(t, ServingFor("image/png").Sandbox)

	// A charset parameter must not defeat the lookup.
	require.True(t, ServingFor("image/png; charset=binary").Inline)
}

func TestCleanFileNameKeepsALabelAndNeverAPath(t *testing.T) {
	require.Equal(t, "report.pdf", CleanFileName("report.pdf", "file"))
	require.Equal(t, "passwd", CleanFileName("../../etc/passwd", "file"))
	require.Equal(t, "evil.png", CleanFileName("evil\x00.png", "file"))
	require.Equal(t, "a b.png", CleanFileName("  a b.png  ", "file"))
	require.Equal(t, "file", CleanFileName("   ", "file"))
	require.Equal(t, "file", CleanFileName("...", "file"))
	require.Equal(t, "笔记.docx", CleanFileName("笔记.docx", "file"))
	require.LessOrEqual(t, len([]rune(CleanFileName(strings.Repeat("x", 500), "file"))), 200)
}

func TestContentDispositionEncodesNamesTheHeaderCannotCarryDirectly(t *testing.T) {
	require.Equal(t, "attachment; filename=report.pdf", ContentDisposition("attachment", "report.pdf"))
	require.Contains(t, ContentDisposition("attachment", "a b.pdf"), `filename="a b.pdf"`)
	// A non-ASCII name must still produce a usable header rather than a
	// truncated or unquoted one.
	cd := ContentDisposition("attachment", "笔记.docx")
	require.Contains(t, cd, "attachment")
	require.Contains(t, cd, "filename*=")
}

func TestDimensionsReadTheHeaderWithoutDecoding(t *testing.T) {
	w, h, ok := Dimensions(pngBytes(t, 32, 18))
	require.True(t, ok)
	require.Equal(t, 32, w)
	require.Equal(t, 18, h)

	_, _, ok = Dimensions([]byte("not an image"))
	require.False(t, ok, "an unreadable file simply has no dimensions")
}

func TestVariantRendersASmallerImageOfTheSameShape(t *testing.T) {
	source := pngBytes(t, 1000, 500)
	data, media, err := Variant(source, 320)
	require.NoError(t, err)
	require.Equal(t, "image/png", media)
	require.Less(t, len(data), len(source))

	w, h, ok := Dimensions(data)
	require.True(t, ok)
	require.Equal(t, 320, w)
	require.Equal(t, 160, h, "the aspect ratio must survive")
}

func TestVariantKeepsTheFormatItWasGiven(t *testing.T) {
	_, media, err := Variant(jpegBytes(t, 900, 300), 320)
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", media)
}

func TestVariantAveragesRatherThanSamples(t *testing.T) {
	// Half black, half white, side by side. Sampling one pixel per output
	// pixel keeps hard black and white; averaging produces grey in the middle.
	img := image.NewRGBA(image.Rect(0, 0, 640, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 640; x++ {
			if x%2 == 0 {
				img.Set(x, y, color.RGBA{A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	data, _, err := Variant(buf.Bytes(), 320)
	require.NoError(t, err)
	out, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	r, g, b, _ := out.At(10, 2).RGBA()
	require.InDelta(t, 0x8080, r, 0x1800, "a black/white check pattern must resolve to grey")
	require.InDelta(t, 0x8080, g, 0x1800)
	require.InDelta(t, 0x8080, b, 0x1800)
}

func TestVariantRefusesWhatItCannotImprove(t *testing.T) {
	// Already small enough.
	_, _, err := Variant(pngBytes(t, 200, 200), 320)
	require.ErrorIs(t, err, ErrNoVariant)

	// A GIF would lose its animation, so it is served whole.
	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!" +
		"\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	_, _, err = Variant(gif, 320)
	require.ErrorIs(t, err, ErrNoVariant)

	// An unoffered width is refused rather than rendered.
	_, _, err = Variant(pngBytes(t, 1000, 100), 400)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNoVariant)

	require.True(t, IsVariantWidth(320))
	require.True(t, IsVariantWidth(800))
	require.True(t, IsVariantWidth(1600))
	require.False(t, IsVariantWidth(0))
	require.False(t, IsVariantWidth(2000))
}
