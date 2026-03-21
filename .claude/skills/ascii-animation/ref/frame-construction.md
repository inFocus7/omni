# Frame Construction Guide (ICG Format)

Step-by-step tutorial for building valid ASCII animation frames in ICG (Indexed Color Grid) format.

---

## Mental Model

Think of a frame as two parallel grids, both `cols` wide and `rows` tall:

1. **`chars`** — a plain text string with `\n`-separated rows. Each row has exactly `cols` characters. No HTML, no entities — raw unicode.
2. **`colors`** — a byte array (base64-encoded in JSON), one byte per cell. Each byte is an index into `class_table`. Length = `cols × rows`.

The frames file wraps these in an ICG structure:
```json
{
  "class_table": ["", "ring", "spark"],
  "frames": [
    { "chars": "  ;;====...\n  ...", "colors": "AABBBCCaaa..." }
  ]
}
```

- `class_table[0]` is always `""` (default text color, no palette class)
- `class_table[1+]` are palette class names that map to colors in `meta.json`

---

## Step-by-Step: Building a Frame

### 1. Draw the art as plain text

Start with a `cols × rows` plain-text canvas. Count characters carefully.

Example: cols=20, rows=5
```


    Hello, World!


```
Row 2: `"    Hello, World!   "` = 4 + 13 + 3 = 20 chars ✓
All other rows: 20 spaces ✓

### 2. Build the color grid

Create a byte array of length `cols × rows`. Each byte indexes into `class_table`.

- Byte `0` = default text color (no palette class)
- Byte `1` = `class_table[1]` (e.g. `"ring"`)
- Byte `2` = `class_table[2]` (e.g. `"spark"`)

For the example above with palette `{"hi": "#ff0"}` and `class_table = ["", "hi"]`:
- Row 2, positions 4–17 ("Hello, World!") get byte `1` (class "hi")
- All other cells get byte `0` (default color)

### 3. Base64-encode the colors

Convert the byte array to a base64 string for JSON serialization.

```python
import base64
colors = bytes([0]*20 + [0]*20 + [0]*4 + [1]*13 + [0]*3 + [0]*20 + [0]*20)
colors_b64 = base64.b64encode(colors).decode('ascii')
```

### 4. Verify dimensions

**Rule: every row must have exactly `cols` characters.**

```python
chars = "\n".join(rows_list)
lines = chars.split('\n')
assert len(lines) == rows
for line in lines:
    assert len(line) == cols
```

**Rule: colors must decode to exactly `cols × rows` bytes.**

```python
color_bytes = base64.b64decode(colors_b64)
assert len(color_bytes) == cols * rows
```

### 5. Assemble the ICG frame

```python
frame = {
    "chars": chars,
    "colors": colors_b64
}
```

### 6. Build the frames file

```json
{
  "class_table": ["", "hi"],
  "frames": [
    { "chars": "...", "colors": "..." }
  ]
}
```

---

## Worked Example: 20×5 Frame with 2 Colors

**Setup:** cols=20, rows=5, palette=`{"title": "#ff6600", "body": "#aaaaaa"}`

**class_table:** `["", "title", "body"]`
- Index 0 = `""` (default)
- Index 1 = `"title"` → `#ff6600`
- Index 2 = `"body"` → `#aaaaaa`

**Target art:**
```
====================
=  DASHBOARD DEMO  =
=  version  1.0.0  =
=  by omni         =
====================
```

**Step 1: Build chars string:**
```python
rows_list = [
    "====================",  # 20 chars ✓
    "=  DASHBOARD DEMO  =",  # 20 chars ✓
    "=  version  1.0.0  =",  # 20 chars ✓
    "=  by omni         =",  # 20 chars ✓
    "====================",  # 20 chars ✓
]
chars = "\n".join(rows_list)
```

**Step 2: Build colors array:**

Row 0: all border chars → index 1 (title)
```python
row0 = [1]*20
```

Row 1: `=` border (title), spaces (default), `DASHBOARD DEMO` (body), spaces (default), `=` border (title)
```python
row1 = [1] + [0]*2 + [2]*14 + [0]*2 + [1]
```

Row 2: similar pattern
```python
row2 = [1] + [0]*2 + [2]*14 + [0]*2 + [1]
```

Row 3: similar pattern
```python
row3 = [1] + [0]*2 + [2]*14 + [0]*2 + [1]
```

Row 4: all border chars → index 1 (title)
```python
row4 = [1]*20
```

```python
import base64
color_data = bytes(row0 + row1 + row2 + row3 + row4)
assert len(color_data) == 100  # 20 × 5
colors_b64 = base64.b64encode(color_data).decode('ascii')
```

**Step 3: Assemble:**
```json
{
  "class_table": ["", "title", "body"],
  "frames": [
    {
      "chars": "====================\n=  DASHBOARD DEMO  =\n=  version  1.0.0  =\n=  by omni         =\n====================",
      "colors": "AQEBAQEBAQEBAQEBAQEBAQEAAAAAAAICAGICAGICAGAAAAAAABAQEBAAAAAIAIAIAIAIAAIAAAAAAAQEBAQEBAQEBAQEBAQEBAQE="
    }
  ]
}
```

---

## Color Resolution Chain

To determine the color for a cell at position (row, col):

1. `colorByte = colors[row * cols + col]`
2. `className = class_table[colorByte]`
3. `cssColor = palette[className]` (from meta.json)
4. If `className` is `""` or not in palette → use default text color

---

## Background Rule

**Default-colored cells (byte 0) are free.** They use no palette class and render in the default text color.

Cells that are spaces AND use default color (byte 0) are background. They cost nothing and render as empty space. This is the single biggest efficiency win — a typical frame is 40–60% spaces.

**When background color IS intentional**: add a palette class (e.g. `"bg"`), assign it a color in `meta.json`, and set the corresponding byte index for space cells. This is valid but increases the color data that must be stored.

---

## Drawing Efficiency

### What makes files smaller (good)
- Large regions of the same color index (compresses well with gzip)
- Many cells with index 0 (default color)
- Few total classes in `class_table` (3–5 is ideal)

### What makes files larger (expensive)
- Alternating color indices per character (poor gzip compression)
- Many unique class_table entries
- Very few default-colored cells

### Rule of thumb
Design the color layout as broad regions first. Fine per-cell color detail compresses poorly. Think of it like painting with a wide brush.

---

## Common Mistakes

### 1. Wrong line width
**Problem:** Row has 44 or 46 chars instead of 45.
**Fix:** Count carefully. Use `len(line)` — no HTML stripping needed since ICG uses plain text.

### 2. Colors length mismatch
**Problem:** Base64 decodes to wrong number of bytes.
**Fix:** Ensure `len(base64.b64decode(colors)) == cols * rows`.

### 3. Color index out of range
**Problem:** A byte value exceeds `len(class_table) - 1`.
**Fix:** Ensure all color bytes are in range `0..len(class_table)-1`.

### 4. class_table[0] not empty string
**Problem:** Validation fails because class_table[0] is not `""`.
**Fix:** class_table[0] MUST always be `""` (the default/no-color entry).

### 5. Invalid class name in class_table
**Problem:** Import fails with "invalid class name".
**Fix:** Class names must match `^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$`. No spaces, no dots, no starting with a digit.

### 6. Using HTML instead of plain text
**Problem:** `chars` contains `<span>` tags or HTML entities.
**Fix:** ICG `chars` is raw unicode text. No HTML, no entities. Use actual characters: `<`, `>`, `&`, etc.

### 7. Missing newlines in chars
**Problem:** Frame displays as one long line.
**Fix:** chars must have exactly `rows - 1` newline characters (`\n`), producing `rows` lines.

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
The color bytes shift correspondingly.

### Blinking / pulsing
Alternate between two states. The `chars` may stay the same while `colors` change (e.g. cycling between color indices).

### Morphing
Gradually transform one shape into another across frames — both `chars` and `colors` change.

### Particle / rain effect
Each frame shifts column content down by one row, with new characters at the top.

### Color cycling
Keep `chars` identical across frames; only change the `colors` bytes. This produces color-only animation with very good gzip compression (chars data is identical).

### Optimal FPS ranges
| Effect | Suggested FPS |
|--------|---------------|
| Slow blink / pulse | 2–4 |
| Smooth rotation | 6–12 |
| Fast spin / glitch | 16–24 |
| Rain / particle | 8–12 |
| Text scroll | 4–8 |

---

## Python Snippet: Build ICG Frames

```python
import base64
import json

def make_icg_frame(rows_list, color_grid, cols, rows):
    """
    Build an ICG frame dict.

    rows_list:   list of strings, each exactly `cols` characters
    color_grid:  list of lists of ints, each row has `cols` entries
    """
    assert len(rows_list) == rows
    assert len(color_grid) == rows
    for i, (line, colors) in enumerate(zip(rows_list, color_grid)):
        assert len(line) == cols, f"Row {i}: {len(line)} chars, expected {cols}"
        assert len(colors) == cols, f"Row {i}: {len(colors)} color entries, expected {cols}"

    chars = "\n".join(rows_list)
    flat_colors = []
    for row in color_grid:
        flat_colors.extend(row)
    colors_b64 = base64.b64encode(bytes(flat_colors)).decode('ascii')

    return {"chars": chars, "colors": colors_b64}


def make_blank_frame(cols, rows):
    """Make a blank frame (all spaces, all default color)."""
    rows_list = [" " * cols] * rows
    color_grid = [[0] * cols] * rows
    return make_icg_frame(rows_list, color_grid, cols, rows)


def make_icg_file(class_table, frames):
    """Build the complete ICG data structure."""
    return {
        "class_table": class_table,
        "frames": frames
    }
```

### Usage example:
```python
class_table = ["", "accent"]
frames = []

# Frame 0: star in center
rows_list = [" " * 10] * 2 + ["    *     "] + [" " * 10] * 2
color_grid = [[0]*10] * 2 + [[0]*4 + [1] + [0]*5] + [[0]*10] * 2
frames.append(make_icg_frame(rows_list, color_grid, 10, 5))

# Frame 1: star moved right
rows_list = [" " * 10] * 2 + ["     *    "] + [" " * 10] * 2
color_grid = [[0]*10] * 2 + [[0]*5 + [1] + [0]*4] + [[0]*10] * 2
frames.append(make_icg_frame(rows_list, color_grid, 10, 5))

icg = make_icg_file(class_table, frames)
with open("frames-1x1.json", "w") as f:
    json.dump(icg, f)
```
