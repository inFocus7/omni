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

This skill creates OMNI-importable ASCII animation folders — either a single animation or a multi-animation pack. It guides the user through a conversation to gather requirements, then generates the correct folder structure, `meta.json`, `frames-*.json`, and optionally `pack.json`. All output validates cleanly against OMNI's import pipeline.

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

### frames-<size>.json
```json
["frame0_html_string", "frame1_html_string", ...]
```
- JSON array of strings, one per frame
- At least 1 frame required

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

**This is the most critical section.** Every frame must be an HTML string satisfying these constraints:

### Dimension rule (MUST follow exactly)
- Each frame = `rows` lines joined by `\n`
- Each line MUST contain exactly `cols` **visible characters**
- Visible chars = text content only, **not counting HTML tag characters**
- Pad short lines with spaces on the right

### Allowed HTML
- `<span class="paletteName">text</span>` — for coloring text
- `<br>` — generates a visual line break (use `\n` for the programmatic line separator instead)
- All other tags are stripped or forbidden

### Forbidden
- `<script>`, `<style>`, `<svg>`, `<iframe>`, `<object>`, `<embed>`, `<math>`, `<template>` — **stripped entirely including content**
- `<br>` should NOT replace `\n` newlines — use `\n` as the line separator

### Practical construction
1. Design the art as a `cols × rows` grid of visible characters
2. For colored regions: wrap the visible characters in `<span class="className">...</span>`
3. Uncolored spaces count as visible characters — always pad to `cols`
4. Join the `rows` line strings with `\n`

### Example (cols=10, rows=3, palette={"hi": "#ff0"})
```
line 0: "          "  (10 spaces)
line 1: "  <span class=\"hi\">hello</span>    "  → visible = 2 + 5 + 3 = 10 ✓
line 2: "          "  (10 spaces)
```
Frame string: `"          \n  <span class=\"hi\">hello</span>    \n          "`

> Worked examples with character counting: see `ref/frame-construction.md`

---

## Background Rule

Plain spaces `" "` are **free** — they cost zero bytes, need no span, and are invisible. This is the default for all empty cells.

**Never span spaces unless the user explicitly asks for a colored background fill.** Spanned spaces are the #1 source of span bloat. Every unneeded background span wastes ~24 bytes per character and adds DOM nodes the browser must render on every frame update.

```
Good:  "     <span class="hi">HELLO</span>     "   ← spaces outside span = free
Bad:   "<span class="bg">     </span><span class="hi">HELLO</span><span class="bg">     </span>"  ← spans on spaces = waste
```

**Optional background color**: if the user explicitly wants a colored fill behind the art, define a `bg` palette class and wrap spaces in `<span class="bg"> </span>`. This is intentional and valid — `validate.py` will not flag it. Just confirm the user asked for it.

---

## Drawing Efficiency

These principles apply **while generating frames**. Following them produces smaller files and faster rendering without sacrificing visual quality.

**1. Group same-color characters together horizontally.**
Consecutive characters of the same palette class collapse into one `<span>` (run-length encoding). Design color regions as horizontal blocks:
- `<span class="mid">############</span>` = 1 span for 12 chars ✓
- `<span class="mid">#</span><span class="mid">#</span>...` = 12 spans for 12 chars ✗

**2. Use 3–5 colors unless the design genuinely needs more.**
Fewer colors = longer runs = fewer spans = smaller file. Before adding a 6th color, ask: does it meaningfully improve the animation, or just add noise?

**3. Prefer wide color regions over fine per-character detail.**
A full row in one color = 1 span. Alternating colors per character = up to `cols` spans. Match color granularity to what the animation actually needs.

**4. Validate always and review the size analysis.**
`validate.py` reports exact gzip size, spans/frame, and run length after writing files. If anything is flagged, offer to revise before the user imports. See the Workflow section for threshold guidance.

### Size recommendation thresholds (from validate.py)

| Signal | Recommendation |
|--------|----------------|
| Avg spans/frame > 400 | Reduce palette colors |
| Avg spans/frame > 400 AND FPS > 12 | Reduce colors AND/OR FPS |
| Avg run length < 3 | Color regions too fragmented — reduce palette or cols×rows |
| Avg run length < 2 | Severe fragmentation — strongly reduce palette |
| Background ratio < 20% | Verify background is unspanned; more cells may be left as plain spaces |
| Overhead ratio > 50% | Tag bytes dominate — reduce palette or unspan more background |
| Gzip > 500 KB | Reduce FPS, cols×rows, or color count |
| Gzip 200–500 KB | Review whether smaller settings are acceptable |
| Frame count > 40 | Consider reducing target FPS (~linear savings) |
| Palette colors > 8 | Above recommended range — monitor span count |

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

The dashboard uses **cover-fit**: the font scales so the animation fills the widget completely, with `overflow: hidden` clipping any excess. This eliminates blank padding around the animation.

**Perfect fit (zero clipping):** when the animation's aspect ratio exactly matches the widget cell, neither dimension overflows:
```
cols * 0.6 / rows  ==  pxW / pxH
```
`fit.py`'s **balanced** suggestion targets this ratio. If you use it, the animation fills the widget with no clipping at any screen size. This is especially important for animation sets that tie into one another visually — design to the balanced option and keep meaningful content away from the outer ~5% of edges as a safe zone.

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
Write `meta.json` and `frames-<size>.json` (and `pack.json` for packs) to the output directory.

### 4. Validate
```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/validate.py <output-dir>
```
Fix any structural errors reported, then re-validate until clean.

After the structural checks, `validate.py` outputs a **size analysis** section with exact gzip size, spans/frame, run length, background ratio, and overhead ratio. Review it:
- If any ⚠⚠ threshold is flagged: proactively offer a specific adjustment with estimated savings before the user imports. Example: *"Your animation is 340 KB gzipped — would you like me to simplify the palette from 6 to 4 colors (estimated ~40% reduction)?"*
- If any ⚠ threshold is flagged: mention it and ask whether to proceed or adjust. Example: *"Average run length is 2.1 — color regions are fragmented. Would you like me to redraw with broader color blocks?"*
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
| `ref/frame-construction.md` | Step-by-step tutorial, worked examples, common mistakes, animation tips |

## Example Files

| Path | Description |
|------|-------------|
| `examples/single-static/` | Minimal 1-frame, no-palette animation |
| `examples/single-animated/` | Multi-frame spinner with palette |
| `examples/sample-pack/` | Pack with 2 animations (wave + dots) |
