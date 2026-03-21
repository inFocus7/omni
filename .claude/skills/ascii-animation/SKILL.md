---
name: ascii-animation
description: Create importable ASCII animations (single or pack) for the OMNI dashboard. Use when users ask to generate, design, or build ASCII art animations.
argument-hint: "[description of desired animation]"
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
---

## Purpose

This skill creates OMNI-importable ASCII animation folders — either a single animation or a multi-animation pack. It guides the user through a conversation to gather requirements, then generates the correct folder structure, `meta.json`, `frames-*.json` (in ICG format), and optionally `pack.json`. All output validates cleanly against OMNI's import pipeline.

---

## Guided Conversation Flow

Work through these questions **before** generating any files. Ask all required questions in one message where possible; don't block on optional ones.

### Step 1 — Format
- **Single animation** or **pack** (multiple animations bundled together)?

### Step 2 — Name(s)
- Animation name(s): lowercase kebab-case, e.g. `spinning-globe`, `plasma-burst`
- For packs: pack name + individual animation names

### Step 3 — Grid Size
Ask the user what dashboard widget size they want. Present the table and let them pick:

| Size | Widget px (approx) | Recommended cols × rows |
|------|-------------------|------------------------|
| 1x1  | 161 × 130         | ~45 × 22               |
| 2x1  | 334 × 130         | ~93 × 22               |
| 1x2  | 161 × 272         | ~45 × 45               |
| 2x2  | 334 × 272         | ~93 × 45               |
| 3x2  | 507 × 272         | ~140 × 45              |

**Then run `fit.py` to compute exact suggestions before generating frames:**
```
python3 ${CLAUDE_SKILL_DIR}/scripts/fit.py <WxH>
```
Present the fit suggestions (keep-rows, keep-cols, balanced) and let the user choose or go with balanced. The table above shows _starting points_ — more cols/rows = smaller font (more detail), fewer = larger font (bolder look).

### Step 4 — Content
- What subject/scene to animate (describe what it looks like)
- Motion style: looping, bouncing, scrolling, blinking, morphing, particle, etc.
- Mood / aesthetic: minimal, retro, glitchy, playful, calm, energetic…

### Step 5 — Color Palette
- Monochrome (no palette) or colored?
- If colored: palette class names + color values
  - Class names: letters/digits/`_`/`-`, start with letter or `_`, max 64 chars
  - Colors: `#RGB`, `#RRGGBB`, `#RRGGBBAA`, `rgb(r,g,b)`, `rgba(r,g,b,a)`, or named CSS color ≤20 chars

### Step 6 — Timing
- FPS (frames per second) — suggest 6–24 FPS. 8 FPS is a good default.
- Frame count — suggest 4–16 frames for smooth loops

### Step 7 — Pack metadata (if pack)
- Author, version (e.g. `1.0.0`), description, license (e.g. `MIT`)

---

## Schema Summary

### meta.json (per animation)
```json
{
  "name": "my-animation",
  "palette": { "colorClass": "#rrggbb" },
  "variants": [
    {
      "size": "1x1",
      "cols": 45,
      "rows": 22,
      "fps": 8,
      "frames_file": "frames-1x1.json"
    }
  ]
}
```
- `name` — required, non-empty
- `palette` — optional; omit entirely if monochrome
- `variants` — required, at least 1 entry
- Each variant: `size`, `cols`, `rows`, `fps`, `frames_file` all required
- Size format: `WxH` (e.g. `1x1`, `2x2`)
- `frames_file` convention: `frames-<size>.json`

### frames-\<size\>.json (ICG format)
```json
{
  "class_table": ["", "ring", "spark"],
  "frames": [
    { "chars": "  ;;====...\n  ...", "colors": "AABBBCCaaa..." }
  ]
}
```
- `class_table[0]` is always `""` (default text color, no palette class)
- `class_table[1+]` are palette class names from `meta.json`
- `chars`: plain text with `\n`-separated rows. Exactly `rows` lines, each exactly `cols` characters. No HTML, no entities — raw unicode.
- `colors`: base64-encoded byte array. Length = `cols × rows`. Each byte indexes into `class_table`.

### pack.json (multi-animation bundle only)
```json
{
  "name": "my-pack",
  "version": "1.0.0",
  "author": "you",
  "description": "Description here",
  "license": "MIT",
  "animations": ["anim-one", "anim-two"]
}
```
- `animations` — required, non-empty array of subdirectory names
- Other fields are optional metadata

> Full schema detail: see `ref/schemas.md`

---

## Frame Construction Rules

**This is the most critical section.** Every frame must have `chars` and `colors` fields satisfying these constraints:

### Chars dimension rule (MUST follow exactly)
- `chars` = `rows` lines of plain text joined by `\n`
- Each line MUST contain exactly `cols` characters
- Characters are raw unicode — no HTML tags, no HTML entities
- Pad short lines with spaces on the right

### Colors rule (MUST follow exactly)
- `colors` = base64-encoded byte array
- Length when decoded = `cols × rows` bytes
- Each byte is an index into `class_table` (0 to len(class_table)-1)
- Byte 0 = default text color (no palette class)

### Color resolution
To get the color for cell at (row, col):
1. `colorByte = colors[row * cols + col]`
2. `className = class_table[colorByte]`
3. `cssColor = palette[className]` (from meta.json)
4. If `className` is `""` or not in palette → use default text color

### Practical construction
1. Design the art as a `cols × rows` grid of visible characters (plain text)
2. Build a parallel `cols × rows` grid of color indices (bytes)
3. Base64-encode the flattened color grid
4. Assemble `{"chars": ..., "colors": ...}`

### Example (cols=10, rows=3, palette={"hi": "#ff0"}, class_table=["", "hi"])
```
chars line 0: "          "  (10 spaces)
chars line 1: "  hello   "  → 2 + 5 + 3 = 10 ✓
chars line 2: "          "  (10 spaces)

colors (30 bytes):
  row 0: [0,0,0,0,0,0,0,0,0,0]
  row 1: [0,0,1,1,1,1,1,0,0,0]  ← "hello" colored with class_table[1]="hi"
  row 2: [0,0,0,0,0,0,0,0,0,0]
```

> Worked examples with byte-level detail: see `ref/frame-construction.md`

---

## Background Rule

Default-colored cells (byte 0) are **free** — they require no palette class and render in the default text color. Space characters with byte 0 are invisible background.

**Never color spaces unless the user explicitly asks for a colored background fill.** Coloring spaces wastes color data and increases file size.

```
Good:  chars="     HELLO     ", colors=[0,0,0,0,0,1,1,1,1,1,0,0,0,0,0]  ← spaces use byte 0
Bad:   chars="     HELLO     ", colors=[2,2,2,2,2,1,1,1,1,1,2,2,2,2,2]  ← spaces use a bg color = waste
```

**Optional background color**: if the user explicitly wants a colored fill, add a `bg` class to `class_table` and palette, then use that index for space cells. This is valid — `validate.py` will not flag it.

---

## Drawing Efficiency

These principles apply **while generating frames**. Following them produces smaller files without sacrificing visual quality.

**1. Group same-color cells together.**
Regions of the same color index compress well with gzip. Design color regions as broad blocks.

**2. Use 3–5 colors unless the design genuinely needs more.**
Fewer classes = simpler color grid = better gzip compression.

**3. Prefer wide color regions over fine per-character detail.**
A full row at one color index compresses much better than alternating indices per cell.

**4. Color cycling is cheap.**
If only the colors change between frames (chars stay the same), gzip compression is very effective because the chars data is identical.

**5. Validate always and review the size analysis.**
`validate.py` reports exact gzip size and metrics after writing files. If anything is flagged, offer to revise before the user imports.

### Size recommendation thresholds (from validate.py)

| Signal | Recommendation |
|--------|----------------|
| Gzip > 500 KB | Reduce FPS, cols×rows, or color count |
| Gzip 200–500 KB | Review whether smaller settings are acceptable |
| Frame count > 40 | Consider reducing target FPS (~linear savings) |
| class_table > 8 entries | Above recommended range — monitor file size |
| Avg colors/frame > 5 KB | Reduce cols×rows or class_table entries |

---

## Sizing Guide

**Grid constants** (from `app.js`):
```
GRID_COL_W = 161 px
GRID_ROW_H = 130 px
GRID_GAP   = 12 px
CHAR_W     = 0.6   (JetBrains Mono character width ratio)
```

**Pixel dimensions for a W×H widget:**
```
pxW = W * 161 + (W-1) * 12
pxH = H * 130 + (H-1) * 12
```

**Ideal cols/rows ratio:**
```
k = (pxW / pxH) / 0.6
```

**Derive cols from rows, or rows from cols:**
```
cols ≈ round(rows * k)
rows ≈ round(cols / k)
```

**Font size formula (px):**
```
fontSize = floor(max(pxW / (cols * 0.6), pxH / rows))
```

The dashboard uses **cover-fit**: the font scales so the animation fills the widget completely, with `overflow: hidden` clipping any excess.

**Perfect fit (zero clipping):** when the animation's aspect ratio exactly matches the widget cell:
```
cols * 0.6 / rows  ==  pxW / pxH
```
`fit.py`'s **balanced** suggestion targets this ratio.

Use `fit.py` to compute suggestions automatically. Always run it before generating frames.

---

## Output Location

- If `$ARGUMENTS` contains a path, write files there
- Otherwise, default to `./ascii-out/<name>/` for a single animation, or `./ascii-out/<pack-name>/` for a pack

---

## Workflow

### 1. Gather requirements
Complete all conversation steps above.

### 2. Compute fit
```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/fit.py <WxH>
# e.g.: python3 ${CLAUDE_SKILL_DIR}/scripts/fit.py 2x1
# e.g.: python3 ${CLAUDE_SKILL_DIR}/scripts/fit.py 2x2 --cols 80
```

### 3. Generate files
Write `meta.json` and `frames-<size>.json` (ICG format) and optionally `pack.json` to the output directory.

**Building ICG frames:**
```python
import base64, json

class_table = ["", "accent"]  # index 0 = default, index 1 = accent color

frames = []
for frame_idx in range(num_frames):
    # Build rows_list: list of strings, each exactly `cols` chars
    rows_list = [...]
    chars = "\n".join(rows_list)

    # Build color grid: list of byte values, length = cols * rows
    color_bytes = [...]  # each byte indexes into class_table
    colors_b64 = base64.b64encode(bytes(color_bytes)).decode('ascii')

    frames.append({"chars": chars, "colors": colors_b64})

icg_data = {"class_table": class_table, "frames": frames}
with open("frames-1x1.json", "w") as f:
    json.dump(icg_data, f)
```

### 4. Validate
```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/validate.py <output-dir>
```
Fix any structural errors reported, then re-validate until clean.

After the structural checks, `validate.py` outputs a **size analysis** section. Review it:
- If any threshold is flagged: proactively offer a specific adjustment with estimated savings before the user imports.
- If all metrics are healthy: report the size and proceed.

### 5. Preview
```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/preview.py <output-dir>
```

### 6. Report to user
Tell the user:
- Where the files were written
- Gzip size from the validate output
- How to import: **OMNI → `/ascii` page → Import → Local folder → select the generated folder**
- On name collision: OMNI will prompt to overwrite

---

## Publishing to GitHub

To share an animation:
1. Push the animation folder to a GitHub repository
2. Others can import by downloading/cloning and using local import
3. Future: OMNI will support direct remote import from GitHub repos via the registry system

---

## Reference Files

| File | Contents |
|------|----------|
| `ref/schemas.md` | Complete field-by-field schema docs, validation regexes, error messages |
| `ref/sizing-guide.md` | Grid math, font scaling, responsive breakpoints, extended size table |
| `ref/frame-construction.md` | Step-by-step ICG frame tutorial, worked examples, common mistakes, animation tips |

## Example Files

| Path | Description |
|------|-------------|
| `examples/single-static/` | Minimal 1-frame, no-palette animation |
| `examples/single-animated/` | Multi-frame spinner with palette |
| `examples/sample-pack/` | Pack with 2 animations (wave + dots) |
