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

Converts a GIF into an OMNI-importable ASCII animation folder. Uses Pillow for
luminance-based character mapping and per-cell color extraction. Output uses
RLE-compressed HTML spans — consecutive same-color characters share one
`<span>`, background cells are plain unspanned spaces.

---

## Prerequisites

```bash
python3 -c "from PIL import Image; print('Pillow OK')"
```

If missing: `pip3 install Pillow --break-system-packages`

---

## Workflow overview

```
gif_info.py          analyze GIF → suggest sizes, detect bg, estimate colors
    ↓
[optional previews]  user chooses settings visually before committing
    ↓  size first → charset second → palette last (each constrains the next)
    ↓
estimate.py          confirm file size before full conversion
    ↓
gif_to_frames.py     full conversion
    ↓
validate.py          structural check + exact size metrics
preview.py           text-mode playback
```

Previews are optional — if the user just wants to convert with defaults, skip
straight to estimate.py. But always offer previews when the user seems
uncertain, asks "what will it look like", or when the GIF has a strong visual
identity worth preserving.

---

## Step 1 — Analyze GIF

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/gif_info.py <gif_path>
```

Present the output, then ask in one message:

```
Your GIF: 1080×1080px, 17 frames, 20 FPS

Best widget sizes by aspect ratio:
  ★ 2x2  delta=18.6%  →  49×24 (11px) | 80×39 (6px) | 74×36 (7px)
  ★ 1x1  delta=19.3%  →  50×24 (5px)  | 80×39 (3px) | 74×36 (3px)

Background: #000000 (87.5%) — will be unspanned spaces
Suggested palette colors: 3 (low-moderate complexity)

Would you like to preview size, charset, and/or palette options before
converting? Or shall I proceed with the recommended settings?

Recommended: 2x2 at 74×36, default charset, auto-detected palette.
```

If the user says proceed → skip to Step 5 (estimate).
If the user wants to preview → work through Steps 2–4 in order.
If the user only wants to preview one dimension (e.g. "just show palette options")
→ skip to that step, using recommended defaults for the others.

---

## Step 2 — Preview: sizes _(optional)_

Run first among previews because size constrains everything else.

```bash
# Get JSON from gif_info for size derivation
GIF_JSON=$(python3 ${CLAUDE_SKILL_DIR}/scripts/gif_info.py <gif_path> --json)

python3 ${CLAUDE_SKILL_DIR}/scripts/preview_compare.py <gif_path> \
  --mode sizes \
  --sizes-json "$GIF_JSON" \
  --chars " .',:;clodxkO0KXN@" \
  --frame 0
```

This derives size candidates directly from the GIF's aspect ratio — no
hardcoded list. Present the output in a visualizer widget (dark bg, monospace,
side-by-side grid). Then use `ask_user_input` to offer clickable choices:

```
Which size would you like?
  ○ 2x2 at 74×36 (7px font) — recommended
  ○ 2x2 at 80×39 (6px font) — more detail
  ○ 2x2 at 49×24 (11px font) — bolder
  ○ 1x1 at 50×24 (5px font) — compact
```

Record the confirmed cols and rows before proceeding.

**Note:** use `--sizes "50x24,74x36,80x39"` instead of `--sizes-json` if you
want to compare a specific custom list not derived from gif_info.

---

## Step 3 — Preview: charsets _(optional)_

Run after size is confirmed. Charset choice is visual — always worth previewing
for portrait/detailed subjects.

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/preview_compare.py <gif_path> \
  --mode charsets \
  --cols <confirmed_cols> --rows <confirmed_rows> \
  --charsets "<option1>|<option2>|<option3>" \
  --frame 0
```

Choose 3–5 charsets to compare based on the GIF's content:

- For high-contrast monochrome (like skulls, portraits): lead with `detailed`
  and `clean`, include `blocks` as a bold option
- For colorful or low-contrast GIFs: lead with `default`, include `minimal`
- Always include the default as one option for reference

Charset strings to pass (pipe-separated, ordered sparse→dense):

| Name     | String                 |
| -------- | ---------------------- |
| default  | `" .',:;clodxkO0KXN@"` |
| detailed | `" .:;=+xX$&#@"`       |
| clean    | `" .,:;=+xX#@"`        |
| blocks   | `" ░▒▓█"`              |
| minimal  | `" .+#@"`              |
| dots     | `" .·:+%#█"`           |

Example for a monochrome portrait:

```bash
--charsets " .',:;clodxkO0KXN@| .:;=+xX$&#@| .,:;=+xX#@| ░▒▓█"
```

Present output in visualizer widget. Use `ask_user_input` for choices.
Record the confirmed charset before proceeding.

**Charset rule:** avoid asymmetric chars like `{}[]()` — they disrupt halftone
texture. Characters that read as density: `. , : ; = + x X # @ % ░ ▒ ▓ █`

---

## Step 4 — Preview: palettes _(optional)_

Run last — after size and charset are confirmed. Palette is the cheapest
preview (char grid rendered once, reused for all options).

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/preview_compare.py <gif_path> \
  --mode palettes \
  --cols <confirmed_cols> --rows <confirmed_rows> \
  --chars "<confirmed_charset>" \
  --palettes "<label>:<hex1>,<hex2>|<label2>:<hex1>,<hex2>" \
  --bg-color "<bg_hex>" \
  --colors <n> \
  --frame 0
```

`auto-detected` is always shown as the first option automatically.
Choose which additional palettes to offer based on context — don't show all
of them every time. For a monochrome GIF, lead with neutral options:

```bash
--palettes "white:#888888,#ffffff|light-grey:#aaaaaa,#dddddd|warm-white:#998870,#fff5e0"
```

For a colorful GIF or if the user has a specific color preference, include
relevant accents. Available named presets (pass whichever fit the context):

| Name       | Hex values (darkest → brightest, bg excluded) |
| ---------- | --------------------------------------------- |
| white      | `#888888, #ffffff`                            |
| light-grey | `#aaaaaa, #dddddd`                            |
| warm-white | `#998870, #fff5e0`                            |
| cool-grey  | `#778899, #ccdde8`                            |
| green      | `#00aa2a, #00ff41`                            |
| amber      | `#aa6600, #ffcc00`                            |
| cyan       | `#007799, #00ccff`                            |
| red        | `#aa1100, #ff3300`                            |
| purple     | `#550088, #cc00ff`                            |

Present output in visualizer widget. Use `ask_user_input` for choices.
If the user picks a palette override, note it — you'll apply it as a
`meta.json` edit after conversion (no need to re-convert).

---

## Step 5 — Estimate file size

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/estimate.py <gif_path> \
  --cols <C> --rows <R> --colors <N> --fps <F> \
  --bg-color "<hex>" \
  --frame first
```

Present the full output. If any ⚠⚠ threshold is flagged, ask the user to
adjust before proceeding. If only ⚠, describe the tradeoff and ask.

---

## Step 6 — Convert

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/gif_to_frames.py <gif_path> \
  --cols <C> --rows <R> --colors <N> --fps <F> \
  --bg-color "<hex>" \
  --char-map "<chars>" \
  --name <animation-name> \
  --out <output-dir>
```

For grayscale: add `--grayscale`, omit `--bg-color`.

---

## Step 7 — Apply palette override (if chosen in Step 4)

If the user chose a palette override in Step 4, edit `meta.json` directly —
no need to re-convert:

```python
import json
meta = json.load(open("output-dir/meta.json"))
meta["palette"] = {"dark": "#888888", "bright": "#ffffff"}
json.dump(meta, open("output-dir/meta.json", "w"), indent=2)
```

Also update the `name` field if the user wants to rename the animation.

---

## Step 8 — Validate + preview

```bash
python3 ${CLAUDE_SKILL_DIR}/scripts/validate.py <output-dir>
python3 ${CLAUDE_SKILL_DIR}/scripts/preview.py <output-dir>
```

Present the size analysis. If any threshold is flagged:

- ⚠⚠: proactively suggest a specific fix ("reduce to 3 colors — ~40% smaller")
- ⚠: mention the tradeoff and ask whether to adjust

---

## Step 9 — Import

OMNI → `/ascii` page → Import → Local folder → select the output directory.
On name collision: OMNI will prompt to overwrite.

---

## Background rule

Background cells are written as plain spaces — no `<span>`. This is the
biggest single span savings. `gif_info.py` auto-detects the background color.
Confirm it in Step 1 — if the background is wrong, all other metrics suffer.

Never span spaces unless the user explicitly wants a visible colored background.

---

## RLE rule (mandatory)

`gif_to_frames.py` enforces RLE automatically:

```
✓  <span class="mid">@@@###</span>       ← 1 span, 6 chars
✗  <span class="mid">@</span><span class="mid">@</span>...  ← 6 spans, 6 chars
```

---

## Size thresholds

| Signal                             | Sev | Action                                         |
| ---------------------------------- | --- | ---------------------------------------------- |
| Avg spans/frame > 400              | ⚠   | Reduce palette colors                          |
| Avg spans/frame > 400 AND FPS > 12 | ⚠⚠  | Reduce colors AND/OR FPS                       |
| Avg run length < 3                 | ⚠   | Too fragmented — reduce palette or cols×rows   |
| Avg run length < 2                 | ⚠⚠  | Severe fragmentation — strongly reduce palette |
| Background ratio < 20%             | ⚠   | Check background color is correct              |
| Overhead ratio > 50%               | ⚠   | Tag bytes dominate — reduce palette            |
| Gzip > 500 KB                      | ⚠⚠  | Reduce FPS, cols×rows, or color count          |
| Gzip 200–500 KB                    | ⚠   | Review if acceptable                           |
| Frame count > 40                   | ⚠   | Consider reducing FPS                          |
| Palette colors > 8                 | ⚠   | Monitor span count                             |

---

## Script reference

| Script                               | Purpose                                            | When              |
| ------------------------------------ | -------------------------------------------------- | ----------------- |
| `gif_info.py`                        | Analyze GIF, suggest sizes, detect bg              | Step 1 — always   |
| `gif_info.py --json`                 | Same but machine-readable for `preview_compare.py` | Step 2            |
| `preview_compare.py --mode sizes`    | Compare cols×rows options                          | Step 2 — optional |
| `preview_compare.py --mode charsets` | Compare character sets                             | Step 3 — optional |
| `preview_compare.py --mode palettes` | Compare color palettes                             | Step 4 — optional |
| `estimate.py`                        | File size estimate                                 | Step 5 — always   |
| `gif_to_frames.py`                   | Full conversion                                    | Step 6 — always   |
| `validate.py`                        | Structural + size validation                       | Step 8 — always   |
| `preview.py`                         | Text-mode frame playback                           | Step 8 — always   |

---

## Output location

- If `$ARGUMENTS` contains a path, use it as the output directory
- Otherwise: `./ascii-out/<animation-name>/`
