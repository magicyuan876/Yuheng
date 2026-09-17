package im

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noisyImage produces a hard-to-compress image so size assertions are honest.
func noisyImage(t *testing.T, w, h int) *image.RGBA {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
	}
	return img
}

func TestCompressReplyImageHitsTarget(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, noisyImage(t, 1920, 1080), &jpeg.Options{Quality: 95}))
	src := buf.Bytes()
	require.Greater(t, len(src), 200*1024, "fixture must start well above target")

	out, err := compressReplyImage(src, 100*1024)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 100*1024)

	decoded, format, err := image.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Greater(t, decoded.Bounds().Dx(), 0)
}

func TestCompressReplyImageFlattensPNGAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.NRGBA{R: 200, G: 10, B: 10, A: 0}) // fully transparent
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	out, err := compressReplyImage(buf.Bytes(), 100*1024)
	require.NoError(t, err)
	decoded, _, err := image.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	r, g, b, _ := decoded.At(10, 10).RGBA()
	// Transparent pixels flatten to white, not black.
	assert.Greater(t, r>>8, uint32(240))
	assert.Greater(t, g>>8, uint32(240))
	assert.Greater(t, b>>8, uint32(240))
}

func TestCompressReplyImageSmallInputStaysSmall(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))))
	out, err := compressReplyImage(buf.Bytes(), 100*1024)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 100*1024)
}

func TestCompressReplyImageRejectsGarbage(t *testing.T) {
	_, err := compressReplyImage([]byte("definitely not an image"), 100*1024)
	assert.Error(t, err)
}

func TestReplyImageTargetBytesEnvOverride(t *testing.T) {
	t.Setenv("IM_REPLY_IMAGE_MAX_KB", "64")
	assert.Equal(t, 64*1024, replyImageTargetBytes())
	t.Setenv("IM_REPLY_IMAGE_MAX_KB", "garbage")
	assert.Equal(t, defaultReplyImageTargetKB*1024, replyImageTargetBytes())
}
