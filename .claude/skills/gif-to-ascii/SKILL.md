---
name: gif-to-ascii
description: Convert a GIF file into an OMNI-importable ASCII animation with color palette. Use when the user provides a GIF and wants to generate an ASCII animation from it.
argument-hint: "<path-to-gif>"
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Glob
  - Grep
---

## Purpose

This skill converts a GIF file into an OMNI-importable ASCII animation folder. It uses `ascii-image-converter` for character density mapping and Pillow for per-cell color extraction. Output uses RLE-compressed HTML spans for efficient storage and render performance — only consecutive characters of the same palette class are grouped into one `<span>`, and background cells are left as plain unspanned spaces.

---

## Prerequisites

At the start of every session, verify both tools are available:

```bash
ascii-image-converter --version
python3 -c "from PIL import Image; print('Pillow OK')"
```

If `ascii-image-converter` is missing:
```
brew install TheZoraiz/ascii-image-converter/ascii-image-converter
```

If Pillow is missing:
```
pip3 install Pillow
```

Stop and show install instructions if either check fails.

---

## Guided Conversation Flow

### Step 1 — GIF path
If not provided via `$ARGUMENTS`, ask for the GIF file path.

### Step 2 — Analyze GIF
Run `gif_info.py` immediately after receiving the path:
```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/gif_info.py <gif_path>
```

### Step 3 — Present findings and ask all questions at once

Present the full `gif_info.py` output to the user, then ask everything in one message:

```
Your GIF: 480×270 px, 18 frames, ~10 FPS (uniform)

Best widget sizes by aspect ratio:
  ★ 2x1  — 334×130 px  (GIF ratio 1.78 vs widget 2.57 — 31% off)
    Options: 81×19 (6px font) | 50×12 (10px font) | 103×24 (5px font)
  ★ 2x2  — 334×272 px  (GIF ratio 1.78 vs widget 1.23 — 45% off)
    Options: 74×36 (7px font) | 49×24 (11px font) | 80×39 (6px font)
  ★ 1x1  — 161×130 px  (GIF ratio 1.78 vs widget 1.24 — 44% off)
    Options: 45×22 (7px font) | 29×14 (11px font) | 57×28 (5px font)

Dominant colors detected (frames 1, 9, 18):
  #1a1a2e  28%  ← likely background
  #4a4e69  19%
  #9a8c98  15%
  #e63329  12%
  #f2e9e4  10%
  + 3 minor colors

Please confirm:
  a) Widget size + cols×rows  (recommend 2x1, 81×19)
  b) Grayscale or colored?
  c) If colored: how many palette colors?  (recommend 5 for this GIF — complexity: moderate)
  d) Background color — is #1a1a2e correct?  (unspanned, saves ~28% of spans)
  e) Character set: default " .',:;clodxkO0KXN" or custom?
  f) Target FPS: accept 10 FPS or override?
  g) Animation name (lowercase-kebab)
```

### Step 4 — Run estimate.py (blocking)

Before full conversion, run a sample-frame estimate with the confirmed settings:

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/estimate.py <gif_path> \
  --cols <C> --rows <R> --colors <N> --fps <F> \
  --bg-color "<hex>" \
  --frame first
```

Present the full output including recommendations. If any ⚠⚠ threshold is flagged, ask the user to adjust settings before proceeding. If only ⚠ thresholds, describe the tradeoff and ask whether to proceed. This step can loop — re-run if settings change.

### Step 5 — Convert

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/gif_to_frames.py <gif_path> \
  --cols <C> --rows <R> --colors <N> --fps <F> \
  --bg-color "<hex>" \
  --char-map "<chars>" \
  --name <animation-name> \
  --out <output-dir>
```

For grayscale: add `--grayscale` and omit `--bg-color`.

### Step 6 — Review generated palette

Present the palette from the script's stdout:

```
Generated palette:
  shadow  → #1a1a2e   (background — unspanned)
  dark    → #4a4e69
  mid     → #9a8c98
  accent  → #e63329
  bright  → #f2e9e4

Want to rename any classes or adjust any colors before import?
```

If the user renames or adjusts, edit `meta.json` directly — no need to re-convert.

### Step 7 — Validate + Preview

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/validate.py <output-dir>
python3 ${CLAUDE_SKILL_DIR}/scripts/preview.py <output-dir>
```

`validate.py` outputs exact gzip size, spans/frame, run length, background ratio, and threshold recommendations. Since the full conversion is done, these are exact measurements. Present the size analysis. If any threshold is flagged, offer specific adjustments:
- ⚠⚠: proactively suggest specific numbers ("reduce to 3 colors — estimated ~40% smaller")
- ⚠: mention and ask whether to proceed or adjust

### Step 8 — Import

OMNI → `/ascii` page → Import → Local folder → select the output directory.
On name collision: OMNI will prompt to overwrite.

---

## Background Rule

Plain `" "` spaces cost nothing. Do not span spaces unless the user explicitly wants a visible colored background fill.

- **Background class** = the most common quantized color class in the GIF. `gif_info.py` auto-detects it (confirmed in Step 3d).
- Background cells are written as plain spaces in the output — no `<span>`.
- This is the single biggest span savings: a GIF with 28% background cells saves 28% of all potential spans.

---

## RLE Rule (mandatory)

`gif_to_frames.py` enforces this automatically — never emit per-character spans:

```
✓  <span class="mid">@@@###</span>       ← 1 span, 6 chars
✗  <span class="mid">@</span><span class="mid">@</span>...  ← 6 spans, 6 chars
```

---

## Palette Size Guidance

- **Recommended**: 3–8 colors
- **Above 8**: warn the user, show `estimate.py` span impact before proceeding
- Fewer colors = longer RLE runs = fewer spans = smaller files + faster rendering

Auto-assigned class names by luminance rank (darkest → brightest):

| Count | Names |
|-------|-------|
| 2 | `dark`, `bright` |
| 3 | `dark`, `mid`, `bright` |
| 4 | `dark`, `mid`, `light`, `bright` |
| 5 | `shadow`, `dark`, `mid`, `light`, `bright` |
| 6 | `shadow`, `dark`, `mid`, `light`, `bright`, `highlight` |
| 7+ | above + `c6`, `c7`, ... |

Background class is excluded from `meta.json` palette (its cells are plain spaces).

---

## Size Recommendation Thresholds

Both `estimate.py` (pre-conversion) and `validate.py` (post-conversion) use these same thresholds:

| Signal | Sev | Recommendation |
|--------|-----|----------------|
| Avg spans/frame > 400 | ⚠ | Reduce palette colors |
| Avg spans/frame > 400 AND FPS > 12 | ⚠⚠ | Reduce colors AND/OR FPS |
| Avg run length < 3 | ⚠ | Color regions too fragmented — reduce palette or cols×rows |
| Avg run length < 2 | ⚠⚠ | Severe fragmentation — strongly reduce palette |
| Background ratio < 20% | ⚠ | Verify background color is correct; more may be unspannable |
| Overhead ratio > 50% | ⚠ | Tag bytes dominate — palette too large or background not unspanned |
| Gzip > 500 KB | ⚠⚠ | Reduce FPS, cols×rows, or color count |
| Gzip 200–500 KB | ⚠ | Review if smaller settings are acceptable |
| Frame count > 40 | ⚠ | Consider reducing FPS (~linear savings) |
| Palette colors > 8 | ⚠ | Above recommended range — monitor span count |

⚠⚠ = proactively suggest specific numbers before proceeding
⚠ = mention tradeoff, ask whether to adjust

---

## Output Location

- If `$ARGUMENTS` contains a path, use it as the output directory
- Otherwise: `./ascii-out/<animation-name>/`

---

## Publishing to GitHub

Push the output folder to a GitHub repository. Others can import by downloading/cloning and using OMNI's local import. Future: OMNI will support direct remote import from GitHub repos.
