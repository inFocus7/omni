package store

import (
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

// NormalizeFrameHTML parses a frame's HTML and normalizes it to exactly
// cols×rows characters. Each line is padded or truncated to cols visible
// characters, and the output has exactly rows newline-separated lines.
// Only <span class="..."> tags are preserved (same as SanitizeFrameHTML).
func NormalizeFrameHTML(input string, cols, rows int) (string, error) {
	// Parse the HTML to extract a grid of {char, class} cells per line.
	r := strings.NewReader("<body>" + input + "</body>")
	doc, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("parse frame HTML: %w", err)
	}

	// Find <body>.
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
	if body == nil {
		return "", fmt.Errorf("no body node found")
	}

	type cell struct {
		ch  rune
		cls string
	}

	// Flatten the body into a single string preserving spans, then split by newline.
	// We walk the DOM and collect cells line-by-line.
	var lines [][]cell
	var currentLine []cell
	var currentCls string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			for _, ch := range n.Data {
				if ch == '\n' {
					lines = append(lines, currentLine)
					currentLine = nil
				} else {
					currentLine = append(currentLine, cell{ch: ch, cls: currentCls})
				}
			}
		case html.ElementNode:
			if n.Data == "span" {
				prevCls := currentCls
				// Extract class attribute.
				for _, attr := range n.Attr {
					if attr.Key == "class" {
						tokens := strings.Fields(attr.Val)
						var valid []string
						for _, tok := range tokens {
							if classNameRe.MatchString(tok) {
								valid = append(valid, tok)
							}
						}
						currentCls = strings.Join(valid, " ")
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				currentCls = prevCls
				return
			}
			if n.Data == "br" {
				lines = append(lines, currentLine)
				currentLine = nil
				return
			}
			// Other elements: recurse into children.
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}

	for c := body.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	// Flush last line.
	lines = append(lines, currentLine)

	// Build normalized output: exactly rows lines, each exactly cols chars.
	var buf strings.Builder
	for row := range rows {
		if row > 0 {
			buf.WriteByte('\n')
		}
		var srcCells []cell
		if row < len(lines) {
			srcCells = lines[row]
		}

		// Truncate or pad to exactly cols cells.
		normalized := make([]cell, cols)
		for c := range cols {
			if c < len(srcCells) {
				normalized[c] = srcCells[c]
			} else {
				normalized[c] = cell{ch: ' ', cls: ""}
			}
		}

		// Serialize with span run-length grouping.
		i := 0
		for i < cols {
			cls := normalized[i].cls
			j := i + 1
			for j < cols && normalized[j].cls == cls {
				j++
			}
			// Collect characters in this run.
			var run strings.Builder
			for k := i; k < j; k++ {
				ch := normalized[k].ch
				switch ch {
				case '&':
					run.WriteString("&amp;")
				case '<':
					run.WriteString("&lt;")
				case '>':
					run.WriteString("&gt;")
				default:
					run.WriteRune(ch)
				}
			}
			if cls != "" {
				buf.WriteString(`<span class="`)
				buf.WriteString(html.EscapeString(cls))
				buf.WriteString(`">`)
				buf.WriteString(run.String())
				buf.WriteString("</span>")
			} else {
				buf.WriteString(run.String())
			}
			i = j
		}
	}

	return buf.String(), nil
}

// WouldTruncateFrames returns true if any frame has visible content outside
// the cols×rows boundary. This is a fast check that short-circuits on the
// first frame with overflow — no normalization or output is produced.
func WouldTruncateFrames(frames []string, cols, rows int) (bool, error) {
	for _, f := range frames {
		truncates, err := wouldTruncateFrame(f, cols, rows)
		if err != nil {
			return false, err
		}
		if truncates {
			return true, nil
		}
	}
	return false, nil
}

// wouldTruncateFrame parses a single frame and checks if any visible
// (non-space) character falls outside cols×rows.
func wouldTruncateFrame(input string, cols, rows int) (bool, error) {
	r := strings.NewReader("<body>" + input + "</body>")
	doc, err := html.Parse(r)
	if err != nil {
		return false, fmt.Errorf("parse frame HTML: %w", err)
	}

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
	if body == nil {
		return false, nil
	}

	lineIdx := 0
	colIdx := 0
	truncates := false

	var walk func(*html.Node) bool
	walk = func(n *html.Node) bool {
		switch n.Type {
		case html.TextNode:
			for _, ch := range n.Data {
				if ch == '\n' {
					lineIdx++
					colIdx = 0
				} else {
					if (lineIdx >= rows || colIdx >= cols) && ch != ' ' {
						truncates = true
						return true
					}
					colIdx++
				}
			}
		case html.ElementNode:
			if n.Data == "br" {
				lineIdx++
				colIdx = 0
				return false
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if walk(c) {
					return true
				}
			}
		}
		return false
	}

	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if walk(c) {
			break
		}
	}

	return truncates, nil
}

// NormalizeFrames normalizes every frame in the slice to exactly cols×rows.
func NormalizeFrames(frames []string, cols, rows int) ([]string, error) {
	out := make([]string, len(frames))
	for i, f := range frames {
		normalized, err := NormalizeFrameHTML(f, cols, rows)
		if err != nil {
			return nil, fmt.Errorf("normalize frame %d: %w", i, err)
		}
		out[i] = normalized
	}
	return out, nil
}

// sanitizeAndRecompressFrames decompresses v.FramesGzip (ICG format),
// validates class names, re-compresses, and regenerates FirstFrame HTML.
func sanitizeAndRecompressFrames(v AnimationVariant) (AnimationVariant, error) {
	icg, err := DecompressICG(v.FramesGzip)
	if err != nil {
		return v, fmt.Errorf("decompress ICG frames: %w", err)
	}

	if err := SanitizeICG(icg); err != nil {
		return v, fmt.Errorf("sanitize ICG: %w", err)
	}

	gz, firstFrame, err := CompressICG(icg)
	if err != nil {
		return v, fmt.Errorf("recompress ICG frames: %w", err)
	}
	v.FramesGzip = gz
	v.FirstFrame = firstFrame
	return v, nil
}
