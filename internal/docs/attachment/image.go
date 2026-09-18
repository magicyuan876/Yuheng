package attachment

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"

	// Registering the decoders is the point of these imports; GIF is decoded
	// for its dimensions only.
	_ "image/gif"
)

// VariantWidths are the sizes a stored image is offered at, chosen to cover a
// thumbnail, a column-width figure and a full-width one on a dense display.
// A request for any other width is refused rather than served, so the set of
// renderings a client can ask the server to compute stays closed.
var VariantWidths = []int{320, 800, 1600}

// MaxImagePixels bounds what will be decoded. A small compressed file can
// declare enormous dimensions, and decoding it would allocate four bytes per
// pixel before anything else had a chance to object.
const MaxImagePixels = 40_000_000

// ErrNoVariant means the image cannot usefully be re-rendered at the
// requested width: the format has no encoder here, or the source is already
// no wider than the target. The caller serves the original.
var ErrNoVariant = fmt.Errorf("docs: no smaller rendering is available")

// Dimensions reads an image's pixel size without decoding it.
// ok is false for a format with no decoder registered (WebP, among others),
// which is not an error: the dimensions are simply unknown.
func Dimensions(data []byte) (width, height int, ok bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// IsVariantWidth reports whether width is one this package will render.
func IsVariantWidth(width int) bool {
	for _, w := range VariantWidths {
		if w == width {
			return true
		}
	}
	return false
}

// Variant renders the image no wider than width, preserving its aspect ratio,
// and returns the encoded bytes with the media type they were encoded as.
//
// Only PNG and JPEG are re-encoded. An animated GIF would lose its animation
// and a WebP cannot be decoded at all, so both are served whole; that is what
// ErrNoVariant says.
func Variant(data []byte, width int) ([]byte, string, error) {
	if !IsVariantWidth(width) {
		return nil, "", fmt.Errorf("docs: %d is not an offered width", width)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("docs: the image could not be read: %w", err)
	}
	if cfg.Width*cfg.Height > MaxImagePixels {
		return nil, "", fmt.Errorf("docs: the image is larger than %d pixels", MaxImagePixels)
	}
	if format != "png" && format != "jpeg" {
		return nil, "", ErrNoVariant
	}
	if cfg.Width <= width {
		return nil, "", ErrNoVariant
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("docs: the image could not be decoded: %w", err)
	}
	bounds := src.Bounds()
	height := bounds.Dy() * width / bounds.Dx()
	if height < 1 {
		height = 1
	}
	dst := downscale(src, width, height)

	var out bytes.Buffer
	if format == "png" {
		if err := png.Encode(&out, dst); err != nil {
			return nil, "", err
		}
		return out.Bytes(), "image/png", nil
	}
	// 85 is the usual point where further quality stops being visible on a
	// photograph while the file keeps growing.
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", err
	}
	return out.Bytes(), "image/jpeg", nil
}

// downscale resamples by averaging every source pixel that falls inside a
// destination pixel's footprint.
//
// Area averaging is the right filter for making an image smaller: it uses all
// the source pixels rather than sampling a few, so it neither aliases fine
// detail into moiré nor softens the result the way a wide reconstruction
// filter would. It is also short enough to read, which is why there is no
// imaging dependency here.
func downscale(src image.Image, dstW, dstH int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	// A fast path for the common case: an RGBA source can be read directly.
	rgba, _ := src.(*image.RGBA)
	if rgba == nil {
		converted := image.NewRGBA(b)
		draw.Draw(converted, b, src, b.Min, draw.Src)
		rgba = converted
	}

	for dy := 0; dy < dstH; dy++ {
		y0 := b.Min.Y + dy*b.Dy()/dstH
		y1 := b.Min.Y + (dy+1)*b.Dy()/dstH
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for dx := 0; dx < dstW; dx++ {
			x0 := b.Min.X + dx*b.Dx()/dstW
			x1 := b.Min.X + (dx+1)*b.Dx()/dstW
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb, sa uint64
			var n uint64
			for y := y0; y < y1; y++ {
				row := rgba.PixOffset(x0, y)
				for x := x0; x < x1; x++ {
					// The values are alpha-premultiplied, which is exactly
					// what makes averaging them correct: a transparent pixel
					// contributes no colour, only transparency.
					sr += uint64(rgba.Pix[row])
					sg += uint64(rgba.Pix[row+1])
					sb += uint64(rgba.Pix[row+2])
					sa += uint64(rgba.Pix[row+3])
					row += 4
					n++
				}
			}
			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: uint8(sa / n),
			})
		}
	}
	return dst
}
