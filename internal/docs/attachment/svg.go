package attachment

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// MaxSVGBytes bounds an SVG upload. The sanitiser rewrites the whole document
// in memory, and a vector drawing that large is a sign of something other than
// a drawing.
const MaxSVGBytes = 4 << 20

// ErrNotSVG is returned when the bytes do not parse as an SVG document.
var ErrNotSVG = fmt.Errorf("docs: not a valid SVG document")

// svgDroppedElements never survive sanitisation. Script is the obvious one;
// the rest are the ways an SVG reaches outside itself — loading a document,
// embedding arbitrary HTML, or animating an attribute into a URL.
var svgDroppedElements = map[string]bool{
	"script": true, "foreignobject": true, "iframe": true, "embed": true,
	"object": true, "handler": true, "set": true, "animate": true,
	"animatetransform": true, "animatemotion": true, "audio": true, "video": true,
}

// svgURLAttributes carry a reference that must be checked before it is kept.
var svgURLAttributes = map[string]bool{
	"href": true, "src": true, "action": true, "formaction": true,
	"xlink:href": true, "from": true, "to": true, "values": true, "by": true,
}

// SanitizeSVG rewrites an SVG document with everything a browser could execute
// removed, and returns the result.
//
// It is a rewrite rather than a scan on purpose: the output is produced by an
// XML encoder from a tree this function built, so anything the parser did not
// understand simply is not in the output. A scanner that deletes known-bad
// substrings has to be right about every encoding trick; a rewriter only has
// to be right about what it keeps.
//
// What survives: elements not on the drop list, attributes that are not event
// handlers, and URL-bearing attributes whose value is a same-document
// fragment, a data: image, or a plain http(s) address. Everything else goes.
func SanitizeSVG(data []byte) ([]byte, error) {
	if len(data) > MaxSVGBytes {
		return nil, fmt.Errorf("docs: SVG is larger than %d bytes", MaxSVGBytes)
	}
	if !looksLikeSVG(data) {
		return nil, ErrNotSVG
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	// Strict, and with only the built-in entities: an external entity is never
	// resolved, so a DOCTYPE cannot be used to read a file off the server.
	decoder.Strict = true
	decoder.Entity = xml.HTMLEntity

	var out bytes.Buffer
	encoder := xml.NewEncoder(&out)

	// depth of the subtree currently being discarded; 0 means "keeping".
	skip := 0
	root := true
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("docs: SVG is not well-formed: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 || svgDroppedElements[strings.ToLower(t.Name.Local)] {
				skip++
				continue
			}
			el := cleanSVGElement(t, root)
			root = false
			if err := encoder.EncodeToken(el); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			t.Name.Space = ""
			if err := encoder.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.CharData:
			if skip > 0 {
				continue
			}
			if err := encoder.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.ProcInst, xml.Comment, xml.Directive:
			// A processing instruction can pull in a stylesheet, a DOCTYPE can
			// declare entities, and a comment can hide a payload for a lenient
			// parser. None of the three draws anything.
			continue
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	if skip != 0 {
		return nil, fmt.Errorf("docs: SVG is not well-formed")
	}
	return out.Bytes(), nil
}

// Namespaces the output declares. Everything else is dropped rather than
// carried through, which is what makes the result canonical: sanitising an
// already-sanitised file returns it unchanged.
const (
	nsSVG   = "http://www.w3.org/2000/svg"
	nsXLink = "http://www.w3.org/1999/xlink"
)

// cleanSVGElement strips one element down to what may be drawn.
//
// Namespace bookkeeping is rebuilt rather than copied. Go's XML encoder
// re-declares a namespace for every element whose name carries one, so passing
// the parsed names through would add an xmlns attribute per element and per
// round trip. Instead the names are flattened and the two namespaces an SVG
// actually uses are declared once, on the root.
func cleanSVGElement(el xml.StartElement, root bool) xml.StartElement {
	el.Name.Space = ""
	kept := el.Attr[:0:0]
	if root {
		kept = append(kept,
			xml.Attr{Name: xml.Name{Local: "xmlns"}, Value: nsSVG},
			xml.Attr{Name: xml.Name{Local: "xmlns:xlink"}, Value: nsXLink},
		)
	}
	for _, a := range el.Attr {
		name := strings.ToLower(a.Name.Local)
		switch {
		case name == "xmlns" || strings.EqualFold(a.Name.Space, "xmlns"):
			// Declarations are re-emitted on the root, never copied.
			continue
		case strings.HasPrefix(name, "on"):
			// Every event handler, without needing to know their names.
			continue
		}
		switch a.Name.Space {
		case "", nsSVG:
			a.Name.Space = ""
		case nsXLink:
			// The one prefixed attribute family SVG uses in practice.
			a.Name = xml.Name{Local: "xlink:" + a.Name.Local}
		default:
			// A namespaced attribute whose prefix cannot be reproduced draws
			// nothing on its own.
			continue
		}
		if svgURLAttributes[name] && !safeSVGURL(a.Value) {
			continue
		}
		if containsScriptURL(a.Value) {
			// style="background:url(javascript:…)" and friends.
			continue
		}
		kept = append(kept, a)
	}
	el.Attr = kept
	return el
}

// safeSVGURL allows a same-document reference, an inline image, or an ordinary
// web address. Everything else — javascript:, data: of a non-image type, and
// any other scheme — is dropped.
func safeSVGURL(v string) bool {
	s := strings.TrimSpace(v)
	if s == "" {
		return true
	}
	lower := strings.ToLower(stripURLWhitespace(s))
	switch {
	case strings.HasPrefix(lower, "#"):
		return true
	case strings.HasPrefix(lower, "data:image/") && !strings.Contains(lower, "svg"):
		// A data: SVG would be a second document this sanitiser never saw.
		return true
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return true
	case !strings.Contains(lower, ":"):
		// A relative path: no scheme, so nothing to execute.
		return true
	default:
		return false
	}
}

// containsScriptURL catches a script scheme anywhere in a value, which is how
// it reaches a CSS property rather than an attribute of its own.
func containsScriptURL(v string) bool {
	lower := strings.ToLower(stripURLWhitespace(v))
	return strings.Contains(lower, "javascript:") || strings.Contains(lower, "vbscript:")
}

// stripURLWhitespace removes the characters a browser ignores when resolving a
// URL, so "java\nscript:" cannot slip past a prefix test.
func stripURLWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v', 0:
			return -1
		default:
			return r
		}
	}, s)
}
