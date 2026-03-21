package store

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestICGFrameJSONMarshal(t *testing.T) {
	colors := []byte{0, 1, 2, 0, 1, 2}
	frame := ICGFrame{Chars: "ab\ncd", Colors: colors}

	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify colors is base64-encoded in JSON.
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	b64 := raw["colors"]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if len(decoded) != len(colors) {
		t.Fatalf("decoded length: want %d, got %d", len(colors), len(decoded))
	}
	for i, b := range decoded {
		if b != colors[i] {
			t.Errorf("byte[%d]: want %d, got %d", i, colors[i], b)
		}
	}
}

func TestICGFrameJSONUnmarshal(t *testing.T) {
	colors := []byte{0, 1, 2, 3}
	b64 := base64.StdEncoding.EncodeToString(colors)
	jsonStr := `{"chars":"ab\ncd","colors":"` + b64 + `"}`

	var frame ICGFrame
	if err := json.Unmarshal([]byte(jsonStr), &frame); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if frame.Chars != "ab\ncd" {
		t.Errorf("chars: want %q, got %q", "ab\ncd", frame.Chars)
	}
	if len(frame.Colors) != len(colors) {
		t.Fatalf("colors length: want %d, got %d", len(colors), len(frame.Colors))
	}
	for i, b := range frame.Colors {
		if b != colors[i] {
			t.Errorf("byte[%d]: want %d, got %d", i, colors[i], b)
		}
	}
}

func TestHTMLFramesToICG_SimpleSpan(t *testing.T) {
	// 5 cols, 1 row: "Hello" with class "fg"
	frame := `<span class="fg">Hello</span>`
	icg, err := HTMLFramesToICG([]string{frame}, 5, 1)
	if err != nil {
		t.Fatalf("HTMLFramesToICG: %v", err)
	}

	if len(icg.ClassTable) < 2 {
		t.Fatalf("class_table too short: %v", icg.ClassTable)
	}
	if icg.ClassTable[0] != "" {
		t.Errorf("class_table[0]: want empty, got %q", icg.ClassTable[0])
	}

	// "fg" should be in the class table.
	fgIdx := -1
	for i, cls := range icg.ClassTable {
		if cls == "fg" {
			fgIdx = i
			break
		}
	}
	if fgIdx == -1 {
		t.Fatalf("class_table missing 'fg': %v", icg.ClassTable)
	}

	if len(icg.Frames) != 1 {
		t.Fatalf("frames count: want 1, got %d", len(icg.Frames))
	}

	f := icg.Frames[0]
	if f.Chars != "Hello" {
		t.Errorf("chars: want %q, got %q", "Hello", f.Chars)
	}
	if len(f.Colors) != 5 {
		t.Fatalf("colors length: want 5, got %d", len(f.Colors))
	}
	for i, b := range f.Colors {
		if int(b) != fgIdx {
			t.Errorf("color[%d]: want %d, got %d", i, fgIdx, b)
		}
	}
}

func TestHTMLFramesToICG_MixedContent(t *testing.T) {
	// "AB CD" where AB is class "a", space is default, CD is class "b"
	frame := `<span class="a">AB</span> <span class="b">CD</span>`
	icg, err := HTMLFramesToICG([]string{frame}, 5, 1)
	if err != nil {
		t.Fatalf("HTMLFramesToICG: %v", err)
	}

	f := icg.Frames[0]
	if f.Chars != "AB CD" {
		t.Errorf("chars: want %q, got %q", "AB CD", f.Chars)
	}

	// Middle char (space) should be default (0)
	if f.Colors[2] != 0 {
		t.Errorf("color[2] (space): want 0, got %d", f.Colors[2])
	}
	// First two should be class "a", last two class "b"
	if f.Colors[0] == 0 {
		t.Error("color[0]: expected non-zero for class 'a'")
	}
	if f.Colors[3] == 0 {
		t.Error("color[3]: expected non-zero for class 'b'")
	}
}

func TestICGFrameToHTML_RoundTrip(t *testing.T) {
	classTable := []string{"", "red", "blue"}
	chars := "AB\nCD"
	colors := []byte{1, 2, 0, 1} // A=red, B=blue, C=default, D=red

	html := ICGFrameToHTML(chars, colors, classTable)

	// The HTML should contain span tags for colored cells.
	if !strings.Contains(html, `class="red"`) {
		t.Errorf("expected class=red in HTML, got: %s", html)
	}
	if !strings.Contains(html, `class="blue"`) {
		t.Errorf("expected class=blue in HTML, got: %s", html)
	}
	if !strings.Contains(html, "A") || !strings.Contains(html, "B") {
		t.Errorf("expected chars A,B in HTML, got: %s", html)
	}
}

func TestCompressDecompressICG_RoundTrip(t *testing.T) {
	original := &ICGData{
		ClassTable: []string{"", "fg", "bg"},
		Frames: []ICGFrame{
			{Chars: "Hello\nWorld", Colors: []byte{0, 1, 2, 1, 0, 0, 1, 2, 1, 0}},
			{Chars: "World\nHello", Colors: []byte{1, 0, 2, 0, 1, 1, 0, 2, 0, 1}},
		},
	}

	gz, firstHTML, err := CompressICG(original)
	if err != nil {
		t.Fatalf("CompressICG: %v", err)
	}
	if firstHTML == "" {
		t.Error("firstHTML: expected non-empty")
	}
	if len(gz) == 0 {
		t.Fatal("gz: expected non-empty")
	}
	// Gzip magic bytes.
	if gz[0] != 0x1f || gz[1] != 0x8b {
		t.Fatalf("not gzip: first bytes %x", gz[:2])
	}

	roundtrip, err := DecompressICG(gz)
	if err != nil {
		t.Fatalf("DecompressICG: %v", err)
	}

	if len(roundtrip.ClassTable) != len(original.ClassTable) {
		t.Fatalf("class_table length: want %d, got %d", len(original.ClassTable), len(roundtrip.ClassTable))
	}
	for i, cls := range original.ClassTable {
		if roundtrip.ClassTable[i] != cls {
			t.Errorf("class_table[%d]: want %q, got %q", i, cls, roundtrip.ClassTable[i])
		}
	}
	if len(roundtrip.Frames) != len(original.Frames) {
		t.Fatalf("frames count: want %d, got %d", len(original.Frames), len(roundtrip.Frames))
	}
	for i, f := range original.Frames {
		rf := roundtrip.Frames[i]
		if rf.Chars != f.Chars {
			t.Errorf("frame[%d].chars: want %q, got %q", i, f.Chars, rf.Chars)
		}
		if len(rf.Colors) != len(f.Colors) {
			t.Errorf("frame[%d].colors length: want %d, got %d", i, len(f.Colors), len(rf.Colors))
		}
	}
}

func TestNormalizeICG_PadsShortRows(t *testing.T) {
	data := &ICGData{
		ClassTable: []string{""},
		Frames: []ICGFrame{
			{Chars: "AB\nC", Colors: []byte{0, 0, 0, 0}},
		},
	}
	result := NormalizeICG(data, 3, 2)
	f := result.Frames[0]

	lines := strings.Split(f.Chars, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// First line should be padded to 3 chars: "AB "
	if len(lines[0]) != 3 {
		t.Errorf("line 0 length: want 3, got %d (%q)", len(lines[0]), lines[0])
	}
	// Second line should be padded to 3 chars: "C  "
	if len(lines[1]) != 3 {
		t.Errorf("line 1 length: want 3, got %d (%q)", len(lines[1]), lines[1])
	}
	// Colors should be 6 bytes (3*2)
	if len(f.Colors) != 6 {
		t.Errorf("colors length: want 6, got %d", len(f.Colors))
	}
}

func TestNormalizeICG_AddsRows(t *testing.T) {
	data := &ICGData{
		ClassTable: []string{""},
		Frames: []ICGFrame{
			{Chars: "AB", Colors: []byte{0, 0}},
		},
	}
	result := NormalizeICG(data, 2, 3)
	f := result.Frames[0]

	lines := strings.Split(f.Chars, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if len(f.Colors) != 6 { // 2*3
		t.Errorf("colors length: want 6, got %d", len(f.Colors))
	}
}

func TestWouldTruncateICG(t *testing.T) {
	data := &ICGData{
		ClassTable: []string{"", "fg"},
		Frames: []ICGFrame{
			{Chars: "ABCDE\n12345", Colors: []byte{1, 1, 1, 1, 1, 0, 0, 0, 0, 0}},
		},
	}

	// Shrinking cols would truncate non-space chars.
	if !WouldTruncateICG(data, 3, 2) {
		t.Error("expected truncation when reducing cols from 5 to 3")
	}

	// Same size should not truncate.
	if WouldTruncateICG(data, 5, 2) {
		t.Error("expected no truncation at same size")
	}

	// Larger size should not truncate.
	if WouldTruncateICG(data, 10, 5) {
		t.Error("expected no truncation when growing")
	}
}

func TestIsICGFormat(t *testing.T) {
	icg := `{"class_table": [""], "frames": []}`
	if !IsICGFormat([]byte(icg)) {
		t.Error("expected ICG format detection for object JSON")
	}

	html := `["<span>f</span>"]`
	if IsICGFormat([]byte(html)) {
		t.Error("expected non-ICG format detection for array JSON")
	}

	empty := ``
	if IsICGFormat([]byte(empty)) {
		t.Error("expected non-ICG format detection for empty input")
	}
}

func TestSanitizeICG_ValidData(t *testing.T) {
	data := &ICGData{
		ClassTable: []string{"", "valid_class", "cls-2"},
		Frames: []ICGFrame{
			{Chars: "AB\nCD", Colors: []byte{0, 1, 2, 0}},
		},
	}
	if err := SanitizeICG(data); err != nil {
		t.Errorf("SanitizeICG: unexpected error: %v", err)
	}
}

func TestSanitizeICG_InvalidClassName(t *testing.T) {
	data := &ICGData{
		ClassTable: []string{"", "1invalid"},
		Frames:     []ICGFrame{{Chars: "A", Colors: []byte{0}}},
	}
	if err := SanitizeICG(data); err == nil {
		t.Error("SanitizeICG: expected error for invalid class name")
	}
}

func TestSanitizeICG_EmptyClassTableIndex0(t *testing.T) {
	// class_table[0] must be "" — if non-empty but class_table has entries, no error
	// (the function only validates class name format, not color byte ranges)
	data := &ICGData{
		ClassTable: []string{"valid"},
		Frames:     []ICGFrame{{Chars: "A", Colors: []byte{0}}},
	}
	// "valid" at index 0 is allowed — SanitizeICG skips index 0 if it's ""
	// but doesn't enforce that index 0 must be "".
	// This is a valid class name so it should pass.
	if err := SanitizeICG(data); err != nil {
		t.Errorf("SanitizeICG: unexpected error: %v", err)
	}
}

func TestResolveICGPalette(t *testing.T) {
	classTable := []string{"", "ring", "spark"}
	palette := map[string]string{"ring": "#ededed", "spark": "#e63329"}

	resolved := ResolveICGPalette(classTable, palette)

	if len(resolved) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resolved))
	}
	if resolved[0] != "" {
		t.Errorf("[0]: want empty, got %q", resolved[0])
	}
	if resolved[1] != "#ededed" {
		t.Errorf("[1]: want #ededed, got %q", resolved[1])
	}
	if resolved[2] != "#e63329" {
		t.Errorf("[2]: want #e63329, got %q", resolved[2])
	}
}

func TestBuildICGWireFormat(t *testing.T) {
	data := &ICGData{
		ClassTable: []string{"", "fg"},
		Frames: []ICGFrame{
			{Chars: "AB\nCD", Colors: []byte{0, 1, 1, 0}},
		},
	}
	palette := map[string]string{"fg": "#ffffff"}

	wire := BuildICGWireFormat(data, palette, 2, 2)

	if wire.Cols != 2 || wire.Rows != 2 {
		t.Errorf("dims: want 2x2, got %dx%d", wire.Cols, wire.Rows)
	}
	if len(wire.Palette) != 2 {
		t.Fatalf("palette length: want 2, got %d", len(wire.Palette))
	}
	if wire.Palette[0] != "" {
		t.Errorf("palette[0]: want empty, got %q", wire.Palette[0])
	}
	if wire.Palette[1] != "#ffffff" {
		t.Errorf("palette[1]: want #ffffff, got %q", wire.Palette[1])
	}
	if len(wire.Frames) != 1 {
		t.Fatalf("frames count: want 1, got %d", len(wire.Frames))
	}
	if wire.Frames[0].Chars != "AB\nCD" {
		t.Errorf("frame chars: want %q, got %q", "AB\nCD", wire.Frames[0].Chars)
	}
}

func TestParseICGFramesFile(t *testing.T) {
	data := ICGData{
		ClassTable: []string{"", "cls"},
		Frames: []ICGFrame{
			{Chars: "AB", Colors: []byte{0, 1}},
		},
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseICGFramesFile(jsonBytes)
	if err != nil {
		t.Fatalf("ParseICGFramesFile: %v", err)
	}
	if len(parsed.Frames) != 1 {
		t.Fatalf("frames: want 1, got %d", len(parsed.Frames))
	}
	if parsed.Frames[0].Chars != "AB" {
		t.Errorf("chars: want %q, got %q", "AB", parsed.Frames[0].Chars)
	}
}

func TestHTMLToICGToHTML_RoundTrip(t *testing.T) {
	// Full round-trip: HTML → ICG → HTML
	// The HTML won't be byte-identical but should be semantically equivalent.
	htmlFrames := []string{
		`<span class="red">Hello</span> <span class="blue">World</span>`,
	}

	icg, err := HTMLFramesToICG(htmlFrames, 11, 1)
	if err != nil {
		t.Fatalf("HTMLFramesToICG: %v", err)
	}

	// Convert back to HTML
	f := icg.Frames[0]
	html := ICGFrameToHTML(f.Chars, f.Colors, icg.ClassTable)

	// Should contain both class names and all text content.
	if !strings.Contains(html, "Hello") {
		t.Errorf("missing 'Hello' in round-tripped HTML: %s", html)
	}
	if !strings.Contains(html, "World") {
		t.Errorf("missing 'World' in round-tripped HTML: %s", html)
	}
	if !strings.Contains(html, `class="red"`) {
		t.Errorf("missing class=red in round-tripped HTML: %s", html)
	}
	if !strings.Contains(html, `class="blue"`) {
		t.Errorf("missing class=blue in round-tripped HTML: %s", html)
	}
}
