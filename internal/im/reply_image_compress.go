package im

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // decode support for stored PNG frames/figures
	"os"
	"strconv"
)

// Outbound IM images are recompressed to a small JPEG so chat replies stay
// light on mobile clients and well inside platform payload caps. The target
// size is configurable via IM_REPLY_IMAGE_MAX_KB (default 100).

const (
	defaultReplyImageTargetKB = 100
	// compressMinQuality is the floor for JPEG quality stepping; below this
	// the image degrades faster than it shrinks and downscaling wins.
	compressMinQuality = 25
	// compressMinEdge stops downscaling before the image becomes unreadable.
	compressMinEdge = 320
)

// replyImageTargetBytes returns the configured outbound size target.
func replyImageTargetBytes() int {
	if raw := os.Getenv("IM_REPLY_IMAGE_MAX_KB"); raw != "" {
		if kb, err := strconv.Atoi(raw); err == nil && kb > 0 {
			return kb * 1024
		}
	}
	return defaultReplyImageTargetKB * 1024
}

// compressReplyImage re-encodes data as a JPEG no larger than targetBytes,
// stepping JPEG quality down first and then box-downscaling when quality
// alone is not enough. Transparency is flattened onto white (JPEG has no
// alpha). Inputs already under target are still normalised to JPEG so the
// outbound format is uniform; if every attempt somehow exceeds the target,
// the smallest attempt is returned rather than failing the reply.
func compressReplyImage(data []byte, targetBytes int) ([]byte, error) {
	if targetBytes <= 0 {
		targetBytes = defaultReplyImageTargetKB * 1024
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	img := flattenToRGBA(src)

	var smallest []byte
	for {
		for _, quality := range []int{80, 65, 50, 35, compressMinQuality} {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
				return nil, fmt.Errorf("encode jpeg: %w", err)
			}
			if smallest == nil || buf.Len() < len(smallest) {
				smallest = append([]byte(nil), buf.Bytes()...)
			}
			if buf.Len() <= targetBytes {
				return smallest, nil
			}
		}

		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		if w/2 < compressMinEdge && h/2 < compressMinEdge {
			return smallest, nil
		}
		img = downscaleBox(img, 2)
	}
}

// flattenToRGBA draws the image onto a white RGBA canvas, discarding alpha.
func flattenToRGBA(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)
	return dst
}

// downscaleBox shrinks img by an integer factor using box (area) averaging —
// good enough for chat-sized thumbnails without pulling in a scaling library.
func downscaleBox(img *image.RGBA, factor int) *image.RGBA {
	if factor < 2 {
		return img
	}
	bounds := img.Bounds()
	w := bounds.Dx() / factor
	h := bounds.Dy() / factor
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	samples := factor * factor
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b int
			for dy := 0; dy < factor; dy++ {
				srcOff := img.PixOffset(bounds.Min.X+x*factor, bounds.Min.Y+y*factor+dy)
				row := img.Pix[srcOff : srcOff+factor*4]
				for dx := 0; dx < factor; dx++ {
					r += int(row[dx*4])
					g += int(row[dx*4+1])
					b += int(row[dx*4+2])
				}
			}
			dstOff := dst.PixOffset(x, y)
			dst.Pix[dstOff] = uint8(r / samples)
			dst.Pix[dstOff+1] = uint8(g / samples)
			dst.Pix[dstOff+2] = uint8(b / samples)
			dst.Pix[dstOff+3] = 0xFF
		}
	}
	return dst
}
