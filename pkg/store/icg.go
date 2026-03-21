package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// ICGFrame is a single frame in ICG (Indexed Color Grid) format.
// Chars is plain text with \n-separated rows. Colors is a byte slice
// where each byte indexes into the parent ICGData.ClassTable.
type ICGFrame struct {
	Chars  string `json:"chars"`
	Colors []byte `json:"-"` // raw bytes; marshaled as base64 via custom methods
}

// icgFrameJSON is the JSON representation with base64-encoded colors.
type icgFrameJSON struct {
	Chars  string `json:"chars"`
	Colors string `json:"colors"`
}

// MarshalJSON encodes Colors as a base64 string.
func (f ICGFrame) MarshalJSON() ([]byte, error) {
	return json.Marshal(icgFrameJSON{
		Chars:  f.Chars,
		Colors: base64.StdEncoding.EncodeToString(f.Colors),
	})
}

// UnmarshalJSON decodes Colors from a base64 string.
func (f *ICGFrame) UnmarshalJSON(data []byte) error {
	var j icgFrameJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	colors, err := base64.StdEncoding.DecodeString(j.Colors)
	if err != nil {
		return fmt.Errorf("decode colors base64: %w", err)
	}
	f.Chars = j.Chars
	f.Colors = colors
	return nil
}

// ICGData holds the class table and all frames for one animation variant.
type ICGData struct {
	ClassTable []string   `json:"class_table"`
	Frames     []ICGFrame `json:"frames"`
}

// HTMLFramesToICG converts legacy HTML []string frames to ICG format.
// It parses each frame's HTML using x/net/html, extracting {char, class}
// cells, and builds a unified class table from all unique classes found.
func HTMLFramesToICG(frames []string, cols, rows int) (*ICGData, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("HTMLFramesToICG: frames must not be empty")
	}

	// First pass: parse all frames into cell grids to discover all classes.
	type parsedFrame struct {
		grid [][]icgCell
	}

	classSet := map[string]bool{"": true}
	parsed := make([]parsedFrame, len(frames))

	for fi, frameHTML := range frames {
		grid, err := htmlToICGCells(frameHTML, cols, rows)
		if err != nil {
			return nil, fmt.Errorf("HTMLFramesToICG frame %d: %w", fi, err)
		}
		for _, row := range grid {
			for _, c := range row {
				if c.cls != "" {
					classSet[c.cls] = true
				}
			}
		}
		parsed[fi] = parsedFrame{grid: grid}
	}

	// Build class table: index 0 is always "" (default/no class).
	classTable := []string{""}
	classIndex := map[string]byte{"": 0}
	for cls := range classSet {
		if cls == "" {
			continue
		}
		if len(classTable) >= 256 {
			return nil, fmt.Errorf("HTMLFramesToICG: too many unique classes (max 255)")
		}
		classIndex[cls] = byte(len(classTable))
		classTable = append(classTable, cls)
	}

	// Second pass: build ICG frames.
	icgFrames := make([]ICGFrame, len(frames))
	for fi, pf := range parsed {
		var charsBuf strings.Builder
		colors := make([]byte, cols*rows)

		for r, row := range pf.grid {
			if r > 0 {
				charsBuf.WriteByte('\n')
			}
			for c, cell := range row {
				ch := cell.ch
				if ch == 0 {
					ch = ' '
				}
				charsBuf.WriteRune(ch)
				idx, ok := classIndex[cell.cls]
				if !ok {
					idx = 0
				}
				colors[r*cols+c] = idx
			}
		}

		icgFrames[fi] = ICGFrame{
			Chars:  charsBuf.String(),
			Colors: colors,
		}
	}

	return &ICGData{
		ClassTable: classTable,
		Frames:     icgFrames,
	}, nil
}

// icgCell is a single character + class pair used during HTML→ICG conversion.
type icgCell struct {
	ch  rune
	cls string
}

// htmlToICGCells parses a single HTML frame and extracts a cols×rows grid of cells.
func htmlToICGCells(input string, cols, rows int) ([][]icgCell, error) {
	r := strings.NewReader("<body>" + input + "</body>")
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse frame HTML: %w", err)
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
		return nil, fmt.Errorf("no body node found")
	}

	var lines [][]icgCell
	var currentLine []icgCell
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
					currentLine = append(currentLine, icgCell{ch: ch, cls: currentCls})
				}
			}
		case html.ElementNode:
			if n.Data == "span" {
				prevCls := currentCls
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
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}

	for c := body.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	lines = append(lines, currentLine)

	// Build normalized grid: exactly rows lines, each exactly cols cells.
	grid := make([][]icgCell, rows)
	for ri := range rows {
		row := make([]icgCell, cols)
		var srcCells []icgCell
		if ri < len(lines) {
			srcCells = lines[ri]
		}
		for ci := range cols {
			if ci < len(srcCells) {
				row[ci] = srcCells[ci]
			} else {
				row[ci] = icgCell{ch: ' ', cls: ""}
			}
		}
		grid[ri] = row
	}

	return grid, nil
}

// ICGFrameToHTML renders a single ICG frame as HTML with span RLE grouping.
func ICGFrameToHTML(chars string, colors []byte, classTable []string) string {
	lines := strings.Split(chars, "\n")
	var buf strings.Builder
	colorIdx := 0

	for li, line := range lines {
		if li > 0 {
			buf.WriteByte('\n')
		}
		runes := []rune(line)
		i := 0
		for i < len(runes) {
			cls := ""
			if colorIdx+i < len(colors) {
				ci := colors[colorIdx+i]
				if int(ci) < len(classTable) {
					cls = classTable[ci]
				}
			}
			// Find run of same class
			j := i + 1
			for j < len(runes) {
				nextCls := ""
				if colorIdx+j < len(colors) {
					ci := colors[colorIdx+j]
					if int(ci) < len(classTable) {
						nextCls = classTable[ci]
					}
				}
				if nextCls != cls {
					break
				}
				j++
			}
			// Collect characters in this run
			var run strings.Builder
			for k := i; k < j; k++ {
				ch := runes[k]
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
		colorIdx += len(runes)
	}

	return buf.String()
}

// CompressICG marshals ICG data to JSON, gzip-compresses it, and generates
// the HTML first frame for SSR. Returns the compressed blob and first frame HTML.
func CompressICG(data *ICGData) (gz []byte, firstFrameHTML string, err error) {
	if len(data.Frames) == 0 {
		return nil, "", fmt.Errorf("CompressICG: frames must not be empty")
	}

	firstFrameHTML = ICGFrameToHTML(data.Frames[0].Chars, data.Frames[0].Colors, data.ClassTable)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, "", fmt.Errorf("CompressICG: marshal: %w", err)
	}
	gz, err = GzipCompress(jsonData)
	if err != nil {
		return nil, "", fmt.Errorf("CompressICG: compress: %w", err)
	}
	return gz, firstFrameHTML, nil
}

// DecompressICG decompresses and unmarshals gzipped ICG data.
func DecompressICG(gz []byte) (*ICGData, error) {
	plain, err := GzipDecompress(gz)
	if err != nil {
		return nil, fmt.Errorf("DecompressICG: decompress: %w", err)
	}
	var data ICGData
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, fmt.Errorf("DecompressICG: unmarshal: %w", err)
	}
	return &data, nil
}

// NormalizeICG pads or truncates every frame to exactly cols×rows.
func NormalizeICG(data *ICGData, cols, rows int) *ICGData {
	out := &ICGData{
		ClassTable: data.ClassTable,
		Frames:     make([]ICGFrame, len(data.Frames)),
	}

	for fi, frame := range data.Frames {
		lines := strings.Split(frame.Chars, "\n")
		newColors := make([]byte, cols*rows)
		var charsBuf strings.Builder

		for r := range rows {
			if r > 0 {
				charsBuf.WriteByte('\n')
			}
			var srcRunes []rune
			if r < len(lines) {
				srcRunes = []rune(lines[r])
			}
			for c := range cols {
				if c < len(srcRunes) {
					charsBuf.WriteRune(srcRunes[c])
					// Copy color from old position
					oldIdx := computeOldColorIndex(lines, r, c)
					if oldIdx < len(frame.Colors) {
						newColors[r*cols+c] = frame.Colors[oldIdx]
					}
				} else {
					charsBuf.WriteByte(' ')
					// newColors[r*cols+c] stays 0 (default class)
				}
			}
		}

		out.Frames[fi] = ICGFrame{
			Chars:  charsBuf.String(),
			Colors: newColors,
		}
	}

	return out
}

// computeOldColorIndex calculates the byte offset into the old Colors array
// for a given row/col, accounting for variable-length Unicode runes per line.
func computeOldColorIndex(lines []string, row, col int) int {
	idx := 0
	for r := 0; r < row && r < len(lines); r++ {
		idx += len([]rune(lines[r]))
	}
	return idx + col
}

// WouldTruncateICG checks if normalizing to cols×rows would lose visible
// (non-space) content in any frame.
func WouldTruncateICG(data *ICGData, cols, rows int) bool {
	for _, frame := range data.Frames {
		lines := strings.Split(frame.Chars, "\n")
		// Check for content beyond rows
		for r := rows; r < len(lines); r++ {
			for _, ch := range lines[r] {
				if ch != ' ' {
					return true
				}
			}
		}
		// Check for content beyond cols in existing rows
		for r := 0; r < len(lines) && r < rows; r++ {
			runes := []rune(lines[r])
			for c := cols; c < len(runes); c++ {
				if runes[c] != ' ' {
					return true
				}
			}
		}
	}
	return false
}

// ResolveICGPalette resolves the class table + palette map into a flat
// color array for the wire format: palette[i] = paletteMap[classTable[i]].
func ResolveICGPalette(classTable []string, palette map[string]string) []string {
	resolved := make([]string, len(classTable))
	for i, cls := range classTable {
		if cls == "" {
			resolved[i] = ""
		} else if color, ok := palette[cls]; ok {
			resolved[i] = color
		} else {
			resolved[i] = ""
		}
	}
	return resolved
}

// ICGWireFormat is the JSON structure sent to the browser.
type ICGWireFormat struct {
	Palette []string       `json:"palette"`
	Cols    int            `json:"cols"`
	Rows    int            `json:"rows"`
	Frames  []ICGWireFrame `json:"frames"`
}

// ICGWireFrame is one frame in the wire format.
type ICGWireFrame struct {
	Chars  string `json:"chars"`
	Colors string `json:"colors"` // base64-encoded
}

// BuildICGWireFormat builds the wire-format JSON from ICG data and animation metadata.
func BuildICGWireFormat(data *ICGData, palette map[string]string, cols, rows int) *ICGWireFormat {
	resolved := ResolveICGPalette(data.ClassTable, palette)
	frames := make([]ICGWireFrame, len(data.Frames))
	for i, f := range data.Frames {
		frames[i] = ICGWireFrame{
			Chars:  f.Chars,
			Colors: base64.StdEncoding.EncodeToString(f.Colors),
		}
	}
	return &ICGWireFormat{
		Palette: resolved,
		Cols:    cols,
		Rows:    rows,
		Frames:  frames,
	}
}

// ParseICGFramesFile parses a frames file in ICG format.
func ParseICGFramesFile(data []byte) (*ICGData, error) {
	var icg ICGData
	if err := json.Unmarshal(data, &icg); err != nil {
		return nil, fmt.Errorf("parse ICG frames file: %w", err)
	}
	if len(icg.ClassTable) == 0 {
		return nil, fmt.Errorf("parse ICG frames file: class_table must not be empty")
	}
	if len(icg.Frames) == 0 {
		return nil, fmt.Errorf("parse ICG frames file: frames must not be empty")
	}
	return &icg, nil
}

// IsICGFormat detects whether JSON data is ICG format (object with class_table)
// vs legacy HTML format (array of strings).
func IsICGFormat(data []byte) bool {
	// Quick heuristic: ICG starts with '{', legacy starts with '['
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// SanitizeICG validates class names in the class table.
func SanitizeICG(data *ICGData) error {
	for i, cls := range data.ClassTable {
		if i == 0 && cls == "" {
			continue // index 0 is always empty
		}
		if cls != "" && !classNameRe.MatchString(cls) {
			return fmt.Errorf("%w: invalid class name %q in class_table[%d]", ErrInvalidInput, cls, i)
		}
	}
	return nil
}
