# Sizing Guide

How to compute optimal `cols` and `rows` for any OMNI dashboard widget size.

---

## Dashboard Grid Constants

From `ui/static/app.js`:

```
GRID_COL_W = 161 px   // width of one grid column
GRID_ROW_H = 130 px   // height of one grid row
GRID_GAP   = 12 px    // gap between cells
CHAR_W     = 0.6      // JetBrains Mono character width-to-height ratio
CHAR_H     = 1.0      // character height ratio (normalized to font size)
```

Font: **JetBrains Mono** monospace, `line-height: 1`.

---

## Pixel Dimension Formulas

For a widget spanning `W` grid columns and `H` grid rows:

```
pxW = W * 161 + (W - 1) * 12
pxH = H * 130 + (H - 1) * 12
```

Examples:

| Size | pxW | pxH |
|------|-----|-----|
| 1x1  | 161 | 130 |
| 2x1  | 334 | 130 |
| 1x2  | 161 | 272 |
| 2x2  | 334 | 272 |
| 3x1  | 507 | 130 |
| 3x2  | 507 | 272 |
| 4x2  | 680 | 272 |
| 5x2  | 853 | 272 |
| 2x3  | 334 | 414 |
| 3x3  | 507 | 414 |

---

## Font Size Formula

Given `cols`, `rows`, and widget pixel dimensions:

```
fontSize = floor(min(pxW / (cols * 0.6), pxH / rows))
```

This is the actual pixel font size rendered in the browser. Larger `fontSize` means larger, bolder characters. Smaller means more detail fits but characters may be hard to read.

---

## Ideal Cols/Rows Ratio

The ideal ratio fills the widget without distorting characters:

```
k = (pxW / pxH) / CHAR_W
  = (pxW / pxH) / 0.6
```

This means:
```
cols ≈ rows * k      (ideal cols for a given rows)
rows ≈ cols / k      (ideal rows for a given cols)
```

---

## Fit Suggestions Algorithm

Given current `cols` (C) and `rows` (R) as anchors:

```
k = (pxW / pxH) / 0.6

# Keep rows, compute cols
keep_rows: cols = round(R * k),  rows = R

# Keep cols, compute rows
keep_cols: cols = C,  rows = round(C / k)

# Balanced (least-squares fit to both)
rowsB = round((k * C + R) / (k*k + 1))
balanced: cols = round(k * rowsB),  rows = rowsB
```

Duplicates (same `cols × rows`) are deduplicated.

---

## Recommended Defaults Table

These assume balanced fit with C=80, R=24 as starting anchors:

| Size | pxW | pxH | k (ratio) | Recommended cols × rows |
|------|-----|-----|-----------|------------------------|
| 1x1  | 161 | 130 | 2.067     | ~45 × 22               |
| 2x1  | 334 | 130 | 4.282     | ~93 × 22               |
| 1x2  | 161 | 272 | 0.987     | ~45 × 45               |
| 2x2  | 334 | 272 | 2.047     | ~93 × 45               |
| 3x1  | 507 | 130 | 6.500     | ~140 × 22              |
| 3x2  | 507 | 272 | 3.107     | ~140 × 45              |
| 4x2  | 680 | 272 | 4.167     | ~185 × 45              |
| 5x2  | 853 | 272 | 5.228     | ~230 × 45              |

These are **starting points**. Adjust based on content needs:
- **More cols/rows** → smaller font, finer detail
- **Fewer cols/rows** → larger font, bolder look

---

## Design Tradeoffs

### Large font (fewer cols/rows)
- Bold, readable characters
- Good for: simple icons, minimal designs, text-heavy animations
- Risk: limited drawing canvas

### Small font (more cols/rows)
- Fine detail, complex art possible
- Good for: detailed illustrations, dense particle effects
- Risk: may be hard to read at small widget sizes

### Rule of thumb
- For pure text animations: use the recommended table values or fewer
- For detailed ASCII art: go 1.5–2× the recommended values
- For pixel-art style: try large even numbers (64×32, 128×64, etc.)

---

## Responsive Breakpoints

OMNI's grid narrows at smaller screens:
- **Desktop**: 5 columns, full grid
- **Tablet** (~768px): 3 columns — widgets beyond column 3 wrap
- **Mobile** (~480px): 2 columns — cols/rows stay the same but widget box shrinks

The animation's `cols` and `rows` don't change — only the CSS `fontSize` scales down with the container via `ResizeObserver`. This means animations always fill their container proportionally.

---

## Using fit.py

```bash
# Show suggestions for 2x1, using defaults (C=80, R=24)
python3 scripts/fit.py 2x1

# Anchor cols at 60, compute rows
python3 scripts/fit.py 2x1 --cols 60

# Anchor rows at 30, compute cols
python3 scripts/fit.py 2x1 --rows 30
```

Output shows keep-rows, keep-cols, and balanced options with their font sizes so you can choose based on the desired look.
