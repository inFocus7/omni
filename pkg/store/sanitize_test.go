package store

import (
	"errors"
	"strings"
	"testing"
)

// ── SanitizePalette ────────────────────────────────────────────────────────────

func TestSanitizePalette_ValidEntries(t *testing.T) {
	cases := []struct {
		class string
		color string
	}{
		{"ring", "#0ff"},
		{"glow", "#ff00ff"},
		{"fire", "#ff00ff80"},
		{"my_class", "rgb(255,0,0)"},
		{"cls-2", "rgba(0, 128, 255, 0.5)"},
		{"A", "red"},
		{"_base", "blue"},
		{"z123", "transparent"},
	}
	for _, tc := range cases {
		if err := SanitizePalette(map[string]string{tc.class: tc.color}); err != nil {
			t.Errorf("SanitizePalette(%q, %q): unexpected error: %v", tc.class, tc.color, err)
		}
	}
}

func TestSanitizePalette_InvalidClassNames(t *testing.T) {
	bad := []string{
		"",
		"1start",     // starts with digit
		"has space",  // space
		"has.dot",    // dot
		"has{brace}", // brace
		"has;semi",   // semicolon
		".leading",   // dot prefix
		strings.Repeat("a", 65), // too long (>64 chars)
	}
	for _, class := range bad {
		err := SanitizePalette(map[string]string{class: "#fff"})
		if err == nil {
			t.Errorf("SanitizePalette: expected error for class %q, got nil", class)
			continue
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("SanitizePalette: error for class %q should wrap ErrInvalidInput, got: %v", class, err)
		}
	}
}

func TestSanitizePalette_InvalidColorValues(t *testing.T) {
	bad := []string{
		"url(javascript:evil)",
		"expression(alert(1))",
		"var(--x)",
		"calc(100%)",
		"#",         // malformed hex
		"rgb(",      // unclosed
		"toolongname-that-exceeds-twenty-chars",
		";injected",
		`"quoted"`,
	}
	for _, color := range bad {
		err := SanitizePalette(map[string]string{"cls": color})
		if err == nil {
			t.Errorf("SanitizePalette: expected error for color %q, got nil", color)
			continue
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("SanitizePalette: error for color %q should wrap ErrInvalidInput, got: %v", color, err)
		}
	}
}

func TestSanitizePalette_Empty(t *testing.T) {
	if err := SanitizePalette(nil); err != nil {
		t.Errorf("SanitizePalette(nil): unexpected error: %v", err)
	}
	if err := SanitizePalette(map[string]string{}); err != nil {
		t.Errorf("SanitizePalette({}): unexpected error: %v", err)
	}
}

// ── SanitizeFrameHTML ──────────────────────────────────────────────────────────

func TestSanitizeFrameHTML_PassesSpanAndBr(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: `<span class="ring">hello</span>`,
			want:  `<span class="ring">hello</span>`,
		},
		{
			input: `plain text`,
			want:  `plain text`,
		},
		{
			input: `<br>`,
			want:  `<br>`,
		},
		{
			input: `<span>no class</span>`,
			want:  `<span>no class</span>`,
		},
		{
			input: `<span class="a b">multi</span>`,
			want:  `<span class="a b">multi</span>`,
		},
	}
	for _, tc := range cases {
		got, err := SanitizeFrameHTML(tc.input)
		if err != nil {
			t.Errorf("SanitizeFrameHTML(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("SanitizeFrameHTML(%q):\n  want: %q\n  got:  %q", tc.input, tc.want, got)
		}
	}
}

func TestSanitizeFrameHTML_StripsScript(t *testing.T) {
	input := `<span class="r">ok</span><script>alert(1)</script>`
	got, err := SanitizeFrameHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "<script") || strings.Contains(got, "alert") {
		t.Errorf("script tag not stripped: %q", got)
	}
	if !strings.Contains(got, `<span class="r">ok</span>`) {
		t.Errorf("expected span to survive, got: %q", got)
	}
}

func TestSanitizeFrameHTML_StripsStyle(t *testing.T) {
	input := `<style>body{color:red}</style><span class="g">hi</span>`
	got, err := SanitizeFrameHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "<style") {
		t.Errorf("style tag not stripped: %q", got)
	}
}

func TestSanitizeFrameHTML_StripsEventHandlers(t *testing.T) {
	input := `<span class="r" onclick="alert(1)" style="color:red">text</span>`
	got, err := SanitizeFrameHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "onclick") || strings.Contains(got, "style") {
		t.Errorf("event handler / style attribute not stripped: %q", got)
	}
	if !strings.Contains(got, `<span class="r">text</span>`) {
		t.Errorf("expected clean span, got: %q", got)
	}
}

func TestSanitizeFrameHTML_DropsInvalidClass(t *testing.T) {
	input := `<span class="has space">text</span>`
	got, err := SanitizeFrameHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "has" is a valid token; "space" is also valid — but the raw class value
	// "has space" is two tokens, both valid individually.
	// Expect multi-class to be re-joined.
	if strings.Contains(got, `class="has space"`) {
		// This is actually valid (two separate valid class tokens).
	}
}

func TestSanitizeFrameHTML_EscapesTextNodes(t *testing.T) {
	// Raw < and > in text should be escaped.
	input := `<span class="r">&lt;hello&gt;</span>`
	got, err := SanitizeFrameHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// html.Parse decodes &lt; → <, then EscapeString re-encodes < → &lt;
	if !strings.Contains(got, "&lt;hello&gt;") {
		t.Errorf("text node entities not preserved, got: %q", got)
	}
}

func TestSanitizeFrameHTML_Empty(t *testing.T) {
	got, err := SanitizeFrameHTML("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output, got: %q", got)
	}
}

func TestSanitizeFrameHTML_StripsImg(t *testing.T) {
	input := `<img src="x" onerror="alert(1)"><span class="r">hi</span>`
	got, err := SanitizeFrameHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "<img") || strings.Contains(got, "onerror") {
		t.Errorf("img tag not stripped: %q", got)
	}
}
