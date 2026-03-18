package store

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	// classNameRe allows standard CSS class names.
	classNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$`)

	// colorValueRe allows safe CSS color formats only.
	colorValueRe = regexp.MustCompile(
		`^(#[0-9a-fA-F]{3,8}` +
			`|rgb\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}\s*\)` +
			`|rgba\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*[0-9.]+\s*\)` +
			`|[a-zA-Z]{1,20})$`,
	)
)

// SanitizePalette validates palette class names and color values.
// Returns an error describing the first invalid entry found.
func SanitizePalette(palette map[string]string) error {
	for class, color := range palette {
		if !classNameRe.MatchString(class) {
			return fmt.Errorf("%w: invalid palette class name %q: must match [a-zA-Z_][a-zA-Z0-9_-]{0,63}", ErrInvalidInput, class)
		}
		if !colorValueRe.MatchString(color) {
			return fmt.Errorf("%w: invalid palette color %q for class %q: must be hex, rgb(), rgba(), or named color", ErrInvalidInput, color, class)
		}
	}
	return nil
}

// SanitizeFrameHTML strips all HTML except <span class="..."> and <br> tags.
// Text nodes are HTML-escaped on output. Returns the sanitized HTML string.
func SanitizeFrameHTML(input string) (string, error) {
	r := strings.NewReader("<body>" + input + "</body>")
	doc, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("parse frame HTML: %w", err)
	}

	var buf strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			buf.WriteString(html.EscapeString(n.Data))

		case html.ElementNode:
			switch n.Data {
			case "span":
				buf.WriteString("<span")
				for _, attr := range n.Attr {
					if attr.Key == "class" {
						// Validate each space-separated class token.
						tokens := strings.Fields(attr.Val)
						valid := make([]string, 0, len(tokens))
						for _, tok := range tokens {
							if classNameRe.MatchString(tok) {
								valid = append(valid, tok)
							}
						}
						if len(valid) > 0 {
							buf.WriteString(` class="`)
							buf.WriteString(strings.Join(valid, " "))
							buf.WriteString(`"`)
						}
					}
				}
				buf.WriteString(">")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				buf.WriteString("</span>")
				return

			case "br":
				buf.WriteString("<br>")
				return

			case "script", "style", "svg", "math", "template", "iframe", "object", "embed":
				// Drop entirely — do not recurse into children.
				return
			}

			// For any other element, recurse into children without the tag.
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}

	// Navigate to the <body> node inserted by html.Parse.
	var body *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)

	if body != nil {
		for c := body.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	return buf.String(), nil
}

// sanitizeAndRecompressFrames decompresses v.FramesGzip, sanitizes each
// frame with SanitizeFrameHTML, re-marshals, re-compresses, and returns an
// updated AnimationVariant with clean FramesGzip and FirstFrame.
func sanitizeAndRecompressFrames(v AnimationVariant) (AnimationVariant, error) {
	plain, err := GzipDecompress(v.FramesGzip)
	if err != nil {
		return v, fmt.Errorf("decompress frames: %w", err)
	}

	var frames []string
	if err := json.Unmarshal(plain, &frames); err != nil {
		return v, fmt.Errorf("unmarshal frames: %w", err)
	}

	for i, frame := range frames {
		sanitized, err := SanitizeFrameHTML(frame)
		if err != nil {
			return v, fmt.Errorf("sanitize frame %d: %w", i, err)
		}
		frames[i] = sanitized
	}

	gz, firstFrame, err := CompressFrames(frames)
	if err != nil {
		return v, fmt.Errorf("recompress frames: %w", err)
	}
	v.FramesGzip = gz
	v.FirstFrame = firstFrame
	return v, nil
}
