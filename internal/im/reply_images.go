package im

import (
	"context"
	"crypto/md5" //nolint:gosec // WeCom's msg_item protocol mandates an MD5 content checksum.
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/storageurl"
)

// Network-independent image delivery: instead of rewriting storage references
// to HTTP URLs (which requires the IM client to be able to reach this
// deployment), the app reads the image bytes itself and hands them to the IM
// platform inline. The platform hosts the content, so rendering works
// regardless of where the client sits relative to Yuheng.
//
// Adapters opt in per platform: WeCom's intelligent-bot stream protocol
// accepts up to 10 inline base64 images (JPG/PNG, ≤10MB each) on the
// finish=true frame.

const (
	// maxReplyImages caps inline images per reply (WeCom msg_item limit).
	maxReplyImages = 10
	// maxReplyImageSourceBytes caps the raw stored image we are willing to
	// decode for recompression. Outbound size is governed separately by
	// IM_REPLY_IMAGE_MAX_KB (default 100KB after JPEG recompression).
	maxReplyImageSourceBytes = 30 * 1024 * 1024
)

// ReplyImage is one inline image attached to an outbound IM reply.
type ReplyImage struct {
	Data []byte
	MD5  string // hex MD5 of Data (required by WeCom's msg_item)
}

// ImageStreamFinalizer is the optional capability of a StreamSender whose
// platform can attach inline images to the final stream frame. Adapters that
// implement it receive extracted reply images; all others keep the plain
// FinalizeStream path (with URL rewriting as the fallback rendering story).
type ImageStreamFinalizer interface {
	FinalizeStreamWithImages(ctx context.Context, incoming *IncomingMessage, streamID string, finalContent string, images []ReplyImage) error
}

// InlineImageCapable marks adapters whose SendReply renders
// ReplyMessage.Images natively (non-streaming path).
type InlineImageCapable interface {
	SupportsInlineImages() bool
}

// ImageURLUploader is the optional capability of an adapter whose platform
// can host image bytes on its own CDN and return a client-reachable URL —
// the best of both worlds: the URL embeds inline in markdown AND rendering
// is independent of the client's network reach to this deployment.
// Returning ("", false) from Available means the capability is not
// configured and callers should fall back to other delivery modes.
type ImageURLUploader interface {
	ImageURLUploadAvailable() bool
	UploadImageForURL(ctx context.Context, img ReplyImage) (string, error)
}

// inlineReplyImagesAsURLs replaces storage-referenced markdown images with
// platform-CDN URLs uploaded via uploader, keeping them inline in the
// markdown body. References that fail extraction or upload keep their
// original markdown so downstream URL rewriting still applies.
func inlineReplyImagesAsURLs(
	ctx context.Context, content string, resolver *storageurl.FileServiceResolver, uploader ImageURLUploader,
) (string, int) {
	if resolver == nil || uploader == nil {
		return content, 0
	}
	replaced := 0
	rewritten := replyImageMarkdownRe.ReplaceAllStringFunc(content, func(match string) string {
		if replaced >= maxReplyImages {
			return match
		}
		sub := replyImageMarkdownRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		ref := sub[1]

		data, err := readReplyImage(ctx, resolver, ref)
		if err != nil {
			logger.Warnf(ctx, "[IM] inline URL image skipped (URL rewrite fallback): ref=%s err=%v", ref, err)
			return match
		}
		sum := md5.Sum(data) //nolint:gosec // dedupe key, not a security control
		img := ReplyImage{Data: data, MD5: hex.EncodeToString(sum[:])}

		cdnURL, err := uploader.UploadImageForURL(ctx, img)
		if err != nil {
			logger.Warnf(ctx, "[IM] image CDN upload failed (URL rewrite fallback): ref=%s err=%v", ref, err)
			return match
		}
		replaced++
		return strings.Replace(match, ref, cdnURL, 1)
	})
	if replaced > 0 {
		logger.Infof(ctx, "[IM] %d reply image(s) embedded via platform CDN URLs", replaced)
	}
	return rewritten, replaced
}

// replyImageMarkdownRe matches a markdown image whose target is an internal
// storage reference (resource:// handle or provider:// path) — the same
// reference grammar storageurl rewriting understands.
var replyImageMarkdownRe = regexp.MustCompile(
	`!\[[^\]]*\]\(\s*(` + storageurl.Pattern.String() + `)\s*\)`,
)

// extractReplyImages pulls storage-referenced markdown images out of content,
// reading their bytes through the tenant's storage resolver. Successfully
// extracted images are replaced with a short text placeholder («图N»); images
// that fail to load, exceed the size cap, are not JPG/PNG, or overflow the
// per-message count keep their original markdown so the URL-rewrite fallback
// still applies to them.
func extractReplyImages(
	ctx context.Context, content string, resolver *storageurl.FileServiceResolver,
) (string, []ReplyImage) {
	if resolver == nil {
		return content, nil
	}

	var images []ReplyImage
	rewritten := replyImageMarkdownRe.ReplaceAllStringFunc(content, func(match string) string {
		if len(images) >= maxReplyImages {
			return match
		}
		sub := replyImageMarkdownRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		ref := sub[1]

		data, err := readReplyImage(ctx, resolver, ref)
		if err != nil {
			logger.Warnf(ctx, "[IM] inline image skipped (falling back to URL rewrite): ref=%s err=%v", ref, err)
			return match
		}

		sum := md5.Sum(data) //nolint:gosec // protocol checksum, not a security control
		images = append(images, ReplyImage{Data: data, MD5: hex.EncodeToString(sum[:])})
		return fmt.Sprintf("【图%d】", len(images))
	})

	return rewritten, images
}

// readReplyImage loads one storage-referenced image and recompresses it to
// the configured outbound size target (IM_REPLY_IMAGE_MAX_KB, default 100KB)
// as a JPEG, keeping chat replies light regardless of the stored original.
func readReplyImage(
	ctx context.Context, resolver *storageurl.FileServiceResolver, ref string,
) ([]byte, error) {
	fileSvc := resolver.ResolveFileService(ref)
	if fileSvc == nil {
		return nil, fmt.Errorf("no file service for reference")
	}
	reader, err := fileSvc.GetFile(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxReplyImageSourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(data) > maxReplyImageSourceBytes {
		return nil, fmt.Errorf("source image exceeds decode cap (%d bytes)", maxReplyImageSourceBytes)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("image is empty")
	}

	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/jpeg") && !strings.HasPrefix(mime, "image/png") {
		return nil, fmt.Errorf("unsupported inline image type %s (JPG/PNG supported)", mime)
	}

	compressed, err := compressReplyImage(data, replyImageTargetBytes())
	if err != nil {
		return nil, fmt.Errorf("compress image: %w", err)
	}
	return compressed, nil
}
