// Package attachment holds the pure decisions an uploaded file needs: what it
// actually is, whether a browser may be allowed to render it inline, what its
// pixel dimensions are, and how to strip the executable parts out of an SVG.
//
// None of it touches the database, the storage backends or HTTP, so every rule
// here is exercised directly by its tests. The service layer calls in, decides
// nothing itself, and stores what comes back.
//
// The guiding rule is that a file's name is never evidence. Everything that
// decides how bytes are served is derived from the bytes; the name survives
// only as a label to show and to download under.
package attachment

import (
	"bytes"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"
)

// Kind is the coarse classification stored on the row and used by the editor
// to pick a node type. It deliberately has few values: the exact media type is
// kept separately, and this is only what the UI switches on.
type Kind string

// Attachment kinds. "diagram" is written by the drawing nodes of a later work
// package; nothing here produces it.
const (
	KindFile    Kind = "file"
	KindImage   Kind = "image"
	KindVideo   Kind = "video"
	KindAudio   Kind = "audio"
	KindDiagram Kind = "diagram"
)

// SniffLimit is how many leading bytes are enough to classify a file.
// http.DetectContentType looks at 512; the SVG probe needs a little more room
// for an XML declaration, a DOCTYPE and a comment before the root element.
const SniffLimit = 4096

// Media types this package names explicitly.
const (
	MediaOctetStream = "application/octet-stream"
	MediaSVG         = "image/svg+xml"
	MediaPDF         = "application/pdf"
)

// Result is what sniffing decided about a file.
type Result struct {
	// Media is the type the bytes say it is.
	Media string
	Kind  Kind
	// Ext is the extension the file should be stored and served under,
	// derived from Media rather than from what the uploader called it.
	Ext string
}

// zipByExt refines the one family that cannot be told apart by content: every
// modern Office document is a zip archive, so the bytes alone say "zip". Using
// the extension here is safe because none of these are ever served inline —
// the refinement only improves the label, never the browser's trust.
var zipByExt = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".odt":  "application/vnd.oasis.opendocument.text",
	".ods":  "application/vnd.oasis.opendocument.spreadsheet",
	".odp":  "application/vnd.oasis.opendocument.presentation",
	".epub": "application/epub+zip",
}

// extByMedia maps a sniffed media type back to the extension to store under.
var extByMedia = map[string]string{
	"image/png":        ".png",
	"image/jpeg":       ".jpg",
	"image/gif":        ".gif",
	"image/webp":       ".webp",
	"image/bmp":        ".bmp",
	"image/tiff":       ".tiff",
	"image/x-icon":     ".ico",
	MediaSVG:           ".svg",
	MediaPDF:           ".pdf",
	"video/mp4":        ".mp4",
	"video/webm":       ".webm",
	"video/quicktime":  ".mov",
	"audio/mpeg":       ".mp3",
	"audio/wave":       ".wav",
	"audio/wav":        ".wav",
	"audio/ogg":        ".ogg",
	"audio/aac":        ".aac",
	"application/zip":  ".zip",
	"text/plain":       ".txt",
	"text/csv":         ".csv",
	"application/json": ".json",
}

// Sniff classifies a file from its leading bytes. The name is consulted for
// exactly two things, both of which are labels and never permissions: telling
// the Office formats apart inside the zip family, and keeping a recognisable
// extension for a type that has none of its own.
func Sniff(head []byte, filename string) Result {
	media := strings.TrimSpace(http.DetectContentType(head))
	if i := strings.IndexByte(media, ';'); i >= 0 {
		media = strings.TrimSpace(media[:i])
	}
	ext := strings.ToLower(filepath.Ext(filename))

	switch {
	case looksLikeSVG(head):
		// DetectContentType calls SVG "text/xml" or "text/plain"; the root
		// element is what actually decides, and getting this right is what
		// routes the file through the sanitiser.
		media = MediaSVG
	case media == "application/zip":
		if refined, ok := zipByExt[ext]; ok {
			media = refined
		}
	case media == "text/plain" && ext == ".csv":
		media = "text/csv"
	case media == "text/plain" && ext == ".md":
		media = "text/markdown"
	case media == "text/plain" && ext == ".json":
		media = "application/json"
	}

	out := Result{Media: media, Kind: kindOf(media), Ext: extByMedia[media]}
	if out.Ext == "" {
		out.Ext = safeExt(ext)
	}
	return out
}

func kindOf(media string) Kind {
	switch {
	case strings.HasPrefix(media, "image/"):
		return KindImage
	case strings.HasPrefix(media, "video/"):
		return KindVideo
	case strings.HasPrefix(media, "audio/"):
		return KindAudio
	default:
		return KindFile
	}
}

// safeExt keeps a short alphanumeric extension and drops anything else, so a
// crafted name cannot put a separator or a second extension into a stored path.
func safeExt(ext string) string {
	if ext == "" || len(ext) > 12 {
		return ""
	}
	for _, r := range ext[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return ""
		}
	}
	return ext
}

// looksLikeSVG reports whether the document's root element is <svg>, skipping
// the byte-order mark, whitespace, an XML declaration, comments and a DOCTYPE.
// Anything else is not an SVG no matter what the file is called.
func looksLikeSVG(data []byte) bool {
	s := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	for {
		s = bytes.TrimLeftFunc(s, unicode.IsSpace)
		switch {
		case bytes.HasPrefix(s, []byte("<?xml")):
			i := bytes.IndexByte(s, '>')
			if i < 0 {
				return false
			}
			s = s[i+1:]
		case bytes.HasPrefix(s, []byte("<!--")):
			i := bytes.Index(s, []byte("-->"))
			if i < 0 {
				return false
			}
			s = s[i+3:]
		case bytes.HasPrefix(s, []byte("<!DOCTYPE")), bytes.HasPrefix(s, []byte("<!doctype")):
			end := bytes.IndexByte(s, '>')
			if end < 0 {
				return false
			}
			// A DOCTYPE may carry an internal subset in brackets, whose own
			// declarations contain '>' characters; the document type ends at
			// the '>' that follows the closing bracket, not the first one.
			if open := bytes.IndexByte(s, '['); open >= 0 && open < end {
				closed := bytes.IndexByte(s[open:], ']')
				if closed < 0 {
					return false
				}
				rest := bytes.IndexByte(s[open+closed:], '>')
				if rest < 0 {
					return false
				}
				end = open + closed + rest
			}
			s = s[end+1:]
		default:
			return hasElement(s, "svg")
		}
	}
}

// hasElement reports whether s starts with the named element's start tag.
func hasElement(s []byte, name string) bool {
	if !bytes.HasPrefix(s, []byte("<"+name)) {
		return false
	}
	rest := s[1+len(name):]
	if len(rest) == 0 {
		return false
	}
	return rest[0] == '>' || rest[0] == '/' || unicode.IsSpace(rune(rest[0]))
}

// Serving describes how one stored file may be handed to a browser.
type Serving struct {
	// ContentType is what to send. It is never the sniffed type for a file a
	// browser would treat as active content.
	ContentType string
	// Inline is true when the file may be displayed in the page rather than
	// downloaded.
	Inline bool
	// Sandbox is true when the response needs a sandboxing Content-Security-
	// Policy: the file is rendered by the browser and can carry markup.
	Sandbox bool
}

// inlineMedia is the closed set of types a browser may render from this
// endpoint. Everything outside it is downloaded as an opaque byte stream,
// which is what keeps an uploaded page from ever running as same-origin code.
var inlineMedia = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"image/bmp": true, "image/x-icon": true,
	MediaSVG: true, MediaPDF: true,
	"video/mp4": true, "video/webm": true, "video/quicktime": true,
	"audio/mpeg": true, "audio/wav": true, "audio/wave": true, "audio/ogg": true, "audio/aac": true,
}

// ServingFor decides how to serve a stored file.
//
// SVG is the one type that is both renderable and capable of carrying script,
// so it is allowed inline only because it was sanitised before being stored,
// and even then it is served under a sandboxing policy. PDF renders in a
// viewer rather than as a document, but it can still reach out, so it gets the
// same treatment.
func ServingFor(media string) Serving {
	media = strings.ToLower(strings.TrimSpace(media))
	if i := strings.IndexByte(media, ';'); i >= 0 {
		media = strings.TrimSpace(media[:i])
	}
	if !inlineMedia[media] {
		return Serving{ContentType: MediaOctetStream, Inline: false, Sandbox: true}
	}
	return Serving{ContentType: media, Inline: true, Sandbox: media == MediaSVG || media == MediaPDF}
}

// SandboxPolicy is the Content-Security-Policy served with user content. It
// gives the response an opaque origin, so even a file that slipped past the
// sanitiser cannot read anything of the site's.
const SandboxPolicy = "sandbox; default-src 'none'; img-src data: blob:; style-src 'unsafe-inline'; base-uri 'none'"

// CleanFileName reduces an uploaded name to something safe to store and to put
// in a Content-Disposition header: the last path segment only, no control
// characters, and bounded in length. It is a label, never a path.
func CleanFileName(raw string, fallback string) string {
	// Some upload flows send a full path; take the base name under either
	// separator before anything else looks at it.
	name := strings.ReplaceAll(raw, "\\", "/")
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r == 0 || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimLeft(strings.TrimSpace(name), ".")
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	const maxRunes = 200
	if r := []rune(name); len(r) > maxRunes {
		name = string(r[:maxRunes])
	}
	return name
}

// ContentDisposition formats the download header for a file name, falling back
// to an ASCII-safe form for names the header encoding cannot carry.
func ContentDisposition(disposition, filename string) string {
	v := mime.FormatMediaType(disposition, map[string]string{"filename": filename})
	if v == "" {
		return disposition
	}
	return v
}
