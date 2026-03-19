# Frame Construction Guide

Step-by-step tutorial for building valid ASCII animation frames.

---

## Mental Model

Think of a frame as a **grid of visible characters**: `cols` wide, `rows` tall.

- Every cell in the grid must be filled — use spaces for empty cells
- The grid is represented as `rows` strings joined by `\n`
- Each string must contain exactly `cols` **visible characters**
- Visible characters = all text content, **not counting HTML tag syntax** (`<`, `>`, attribute text)

The frames file is a JSON array of these strings: `["frame0", "frame1", ...]`

---

## Step-by-Step: Building a Frame

### 1. Draw the art as plain text

Start with a `cols × rows` plain-text canvas. Count characters carefully.

Example: cols=20, rows=5
```


    Hello, World!


```
Row 2: "    Hello, World!   " = 4 + 13 + 3 = 20 chars ✓
All other rows: 20 spaces ✓

### 2. Add color spans

Wrap colored regions in `<span class="paletteName">...</span>`.

The visible character count is computed from text content only — the tag itself is invisible.

```
row 2: "    <span class="hi">Hello</span>, World!   "
```
Visible count: 4 spaces + "Hello" (5) + ", World!" (8) + 3 spaces = 20 ✓

Tags `<span class="hi">` and `</span>` are NOT counted.

### 3. Verify each row

**Rule: every row must have exactly `cols` visible characters.**

To count visible characters in a row: strip all HTML tags (`<...>`), then count the remaining characters. Also decode HTML entities before counting:
- `&amp;` → `&` (1 char)
- `&lt;` → `<` (1 char)
- `&gt;` → `>` (1 char)
- `&nbsp;` → ` ` (1 char)
- `&#N;` or `&#xHH;` → 1 char

### 4. Join rows with \n

```python
frame = "\n".join(rows)  # rows is a list of row strings
```

### 5. Include in JSON array

```json
["frame0_string", "frame1_string"]
```

Make sure to escape special characters in JSON strings:
- `"` → `\"`
- `\` → `\\`
- Newlines in the string: these should already be `\n` (literal newline escape, not a raw newline)

---

## Worked Example: 20×5 Frame with 2 Colors

**Setup:** cols=20, rows=5, palette={"title": "#ff6600", "body": "#aaaaaa"}

**Target art:**
```
====================
= DASHBOARD DEMO  =
= version  1.0.0  =
= by omni         =
====================
```

**Step 1: Draw rows and count:**
- row 0: `"===================="` → 20 chars ✓
- row 1: `"= DASHBOARD DEMO  ="` → 20 chars ✓ (1+1+14+3+1=20)
- row 2: `"= version  1.0.0  ="` → 20 chars ✓
- row 3: `"= by omni         ="` → 20 chars ✓
- row 4: `"===================="` → 20 chars ✓

**Step 2: Add color spans:**
```
row 0: "<span class=\"title\">===================</span>="
  visible: 19 + 1 = 20 ✓  ← spans only wrap chars we want colored
```

Wait — let me wrap the full border line:
```
row 0: "<span class=\"title\">====================</span>"
  visible: 20 ✓
row 1: "<span class=\"title\">=</span> <span class=\"body\">DASHBOARD DEMO</span>  <span class=\"title\">=</span>"
  visible: 1 + 1 + 14 + 2 + 1 = 19... ← need to recount
```

Adjusted for exact 20 chars on row 1:
`"= DASHBOARD DEMO  ="` = `=`(1) + ` `(1) + `DASHBOARD DEMO`(14) + `  `(2) + `=`(1) = 19. Hmm, that's 19. Let me use `"= DASHBOARD DEMO  = "` — wait that's 21. Let me use `"=  DASHBOARD DEMO  ="` = 1+2+14+2+1 = 20 ✓.

```json
"<span class=\"title\">=</span>  <span class=\"body\">DASHBOARD DEMO</span>  <span class=\"title\">=</span>"
```
visible: 1 + 2 + 14 + 2 + 1 = 20 ✓

**Step 3: Assemble the frame string:**
```json
"<span class=\"title\">====================</span>\n<span class=\"title\">=</span>  <span class=\"body\">DASHBOARD DEMO</span>  <span class=\"title\">=</span>\n..."
```

---

## Common Mistakes

### 1. Wrong line width
**Problem:** Row has 44 or 46 chars instead of 45.
**Fix:** Count carefully. Use Python: `len(re.sub(r'<[^>]+>', '', row))` to count visible chars.

### 2. Using `<br>` instead of `\n`
**Problem:** Frame displays as one long line.
**Fix:** Use `\n` (the string escape) as the line separator when joining rows. `<br>` is for inline visual breaks within a line and is separate from the row structure.

### 3. Forgetting to pad with spaces
**Problem:** Rows are shorter than `cols` → import validator rejects.
**Fix:** Right-pad every row: `row.ljust(cols)` in Python.

### 4. Counting HTML tag characters as visible
**Problem:** Row looks right visually but validation fails.
**Fix:** Strip all `<tag>` and `</tag>` sequences before counting.

### 5. Invalid palette class name
**Problem:** Import fails with "invalid palette class name".
**Fix:** Class names must match `^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$`. No spaces, no dots, no starting with a digit.

### 6. Backslash not escaped in JSON
**Problem:** JSON parse error on `\` characters in frames.
**Fix:** In JSON strings, `\` must be written as `\\`. The backslash character `\` is `\\` in JSON.

---

## Animation Techniques

### Looping rotation
Repeat a set of frames that seamlessly loop. Frame N+1 = Frame N shifted by one step.

Example — horizontal scroll (line shifts right by 1 each frame):
```
frame 0 row: "~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  "  (45 chars)
frame 1 row: "  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~ "  (45 chars)
frame 2 row: " ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  ~  "  (45 chars)
```

### Blinking / pulsing
Alternate between two states (on/off, bright/dim, expanded/contracted):
```
frame 0: center dot only
frame 1: dot + ring around it
frame 2: center dot only  ← repeat
```

### Morphing
Gradually transform one shape into another across frames:
```
frame 0: [ ]   (box outline)
frame 1: [o]   (dot inside)
frame 2: [O]   (bigger dot)
frame 3: [X]   (filled)
```

### Particle / rain effect
Each frame shifts column content down by one row, with new characters generated at the top:
```
frame 0: top row has random chars, rest empty
frame 1: those chars moved down one row, new chars at top
```

### Optimal FPS ranges
| Effect | Suggested FPS |
|--------|---------------|
| Slow blink / pulse | 2–4 |
| Smooth rotation | 6–12 |
| Fast spin / glitch | 16–24 |
| Rain / particle | 8–12 |
| Text scroll | 4–8 |

---

## HTML Entities in Frames

You can use HTML entities for special characters:

| Entity | Character | Visible count |
|--------|-----------|---------------|
| `&amp;` | `&` | 1 |
| `&lt;` | `<` | 1 |
| `&gt;` | `>` | 1 |
| `&nbsp;` | non-breaking space | 1 |
| `&#9608;` | `█` (block) | 1 |
| `&#9617;` | `░` (light shade) | 1 |
| `&#9618;` | `▒` (medium shade) | 1 |
| `&#9619;` | `▓` (dark shade) | 1 |

Note: The sanitizer HTML-escapes text nodes, so raw `<` and `>` in text get converted to `&lt;` and `&gt;` automatically. Write them as entities if you want them as visible characters.

---

## Python Snippet: Build a Frame

```python
import re

def visible_len(html_str):
    """Count visible characters in an HTML frame row."""
    text = re.sub(r'<[^>]+>', '', html_str)
    # Decode common entities
    text = text.replace('&amp;', '&').replace('&lt;', '<').replace('&gt;', '>')
    text = text.replace('&nbsp;', ' ')
    text = re.sub(r'&#\d+;', 'X', text)   # count each entity as 1 char
    text = re.sub(r'&#x[0-9a-fA-F]+;', 'X', text)
    return len(text)

def make_frame(rows_list):
    """Join rows into a frame string."""
    return '\n'.join(rows_list)

def pad_row(row_html, cols):
    """Right-pad a row to exactly cols visible characters."""
    vlen = visible_len(row_html)
    if vlen < cols:
        return row_html + ' ' * (cols - vlen)
    return row_html
```
