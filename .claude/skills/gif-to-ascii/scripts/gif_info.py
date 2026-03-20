#!/usr/bin/env python3
"""
gif_info.py — Analyze a GIF file and suggest widget sizes + palette for OMNI.

Usage:
  gif_info.py <gif_path>

Output: structured text the agent can parse and present to the user.
"""

import math
import os
import statistics
import sys

try:
    from PIL import Image
except ImportError:
    print("Error: Pillow is required. Install with: pip3 install Pillow", file=sys.stderr)
    sys.exit(1)


# ── Grid constants (match app.js) ────────────────────────────────────────────

GRID_COL_W = 161
GRID_ROW_H = 130
GRID_GAP   = 12
CHAR_W     = 0.6

# All standard widget sizes to check
WIDGET_SIZES = [
    (1, 1), (2, 1), (1, 2), (2, 2), (3, 1), (3, 2),
    (4, 2), (5, 2), (2, 3), (3, 3),
]

DEFAULT_C = 80
DEFAULT_R = 24


# ── Fit computation (mirrors computeFitSuggestions in app.js) ────────────────

def pixel_dims(W, H):
    return (W * GRID_COL_W + (W - 1) * GRID_GAP,
            H * GRID_ROW_H + (H - 1) * GRID_GAP)


def font_size(cols, rows, pxW, pxH):
    if cols <= 0 or rows <= 0:
        return 0
    return math.floor(min(pxW / (cols * CHAR_W), pxH / rows))


def fit_suggestions(W, H, C=DEFAULT_C, R=DEFAULT_R):
    pxW, pxH = pixel_dims(W, H)
    k = (pxW / pxH) / CHAR_W
    rowsB = max(1, round((k * C + R) / (k * k + 1)))
    candidates = [
        (max(1, round(R * k)), R),
        (C, max(1, round(C / k))),
        (max(1, round(k * rowsB)), rowsB),
    ]
    seen = set()
    unique = []
    for cols, rows in candidates:
        key = (cols, rows)
        if key not in seen:
            seen.add(key)
            fs = font_size(cols, rows, pxW, pxH)
            unique.append((cols, rows, fs))
    return unique  # list of (cols, rows, font_px)


# ── Color helpers ─────────────────────────────────────────────────────────────

def rgb_to_hex(r, g, b):
    return f"#{r:02x}{g:02x}{b:02x}"


def color_distance(c1, c2):
    return math.sqrt(sum((a - b) ** 2 for a, b in zip(c1, c2)))


def quantize_frame(frame_img, n_colors=8, sample_size=16):
    """
    Resize frame to sample_size×sample_size, quantize to n_colors,
    return list of (hex_color, count) sorted by count descending.
    """
    small = frame_img.convert('RGB').resize((sample_size, sample_size), Image.Resampling.LANCZOS)
    quantized = small.quantize(colors=n_colors, method=Image.Quantize.MEDIANCUT)
    palette_raw = quantized.getpalette()  # flat list [r,g,b, r,g,b, ...]
    # Count pixels per palette index
    counts = {}
    for px in quantized.getdata():
        counts[px] = counts.get(px, 0) + 1
    total = sample_size * sample_size
    results = []
    for idx, cnt in sorted(counts.items(), key=lambda x: -x[1]):
        r = palette_raw[idx * 3]
        g = palette_raw[idx * 3 + 1]
        b = palette_raw[idx * 3 + 2]
        results.append((rgb_to_hex(r, g, b), cnt / total))
    return results


def aggregate_colors(color_lists, n_final=8):
    """
    Merge color frequency lists from multiple frames.
    Aggregate by approximate color similarity (within distance 30).
    Returns list of (hex, frequency) sorted by frequency desc.
    """
    buckets = []  # list of [hex, total_weight]
    for color_list in color_lists:
        for hex_col, freq in color_list:
            r = int(hex_col[1:3], 16)
            g = int(hex_col[3:5], 16)
            b = int(hex_col[5:7], 16)
            rgb = (r, g, b)
            # Find existing bucket within distance threshold
            matched = None
            for bucket in buckets:
                br = int(bucket[0][1:3], 16)
                bg_ = int(bucket[0][3:5], 16)
                bb = int(bucket[0][5:7], 16)
                if color_distance(rgb, (br, bg_, bb)) < 30:
                    matched = bucket
                    break
            if matched:
                matched[1] += freq
            else:
                buckets.append([hex_col, freq])
    # Normalize and sort
    total = sum(b[1] for b in buckets) or 1
    result = sorted([(b[0], b[1] / total) for b in buckets], key=lambda x: -x[1])
    return result[:n_final]


def color_complexity(color_freqs):
    """
    0.0 = monochrome (one color dominates), 1.0 = uniform spread.
    Based on normalized entropy of the frequency distribution.
    """
    if not color_freqs:
        return 0.0
    freqs = [f for _, f in color_freqs if f > 0]
    if len(freqs) <= 1:
        return 0.0
    entropy = -sum(f * math.log(f) for f in freqs if f > 0)
    max_entropy = math.log(len(freqs))
    return entropy / max_entropy if max_entropy > 0 else 0.0


def suggest_color_count(complexity):
    """Map complexity score to a suggested palette color count."""
    if complexity < 0.25:
        return 2, "low"
    elif complexity < 0.45:
        return 3, "low-moderate"
    elif complexity < 0.65:
        return 5, "moderate"
    elif complexity < 0.80:
        return 6, "moderate-high"
    else:
        return 8, "high"


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    import argparse
    parser = argparse.ArgumentParser(description="Analyze a GIF for OMNI ASCII conversion.")
    parser.add_argument("gif_path", help="Path to the GIF file")
    parser.add_argument("--json", action="store_true",
                        help="Output machine-readable JSON for use with preview_compare.py --sizes-json")
    args = parser.parse_args()

    gif_path = args.gif_path
    if not os.path.isfile(gif_path):
        print(f"Error: file not found: {gif_path}", file=sys.stderr)
        sys.exit(1)

    try:
        img = Image.open(gif_path)
    except Exception as e:
        print(f"Error: cannot open image: {e}", file=sys.stderr)
        sys.exit(1)

    # ── Basic metadata ────────────────────────────────────────────────────────
    width, height = img.size
    n_frames = getattr(img, 'n_frames', 1)

    # Frame durations (in ms)
    durations = []
    try:
        for i in range(n_frames):
            img.seek(i)
            d = img.info.get('duration', 100)
            durations.append(d)
    except EOFError:
        pass
    img.seek(0)

    if durations:
        median_dur = statistics.median(durations)
        fps = round(1000 / median_dur, 1) if median_dur > 0 else 10.0
        uniform = (max(durations) - min(durations)) < 20  # within 20ms
    else:
        fps = 10.0
        median_dur = 100
        uniform = True

    aspect = width / height if height else 1.0

    # ── Widget size matching ───────────────────────────────────────────────────
    matches = []
    for W, H in WIDGET_SIZES:
        pxW, pxH = pixel_dims(W, H)
        widget_ratio = pxW / pxH if pxH else 1.0
        delta = abs(aspect - widget_ratio) / widget_ratio if widget_ratio else 1.0
        suggestions = fit_suggestions(W, H)
        matches.append((delta, W, H, pxW, pxH, widget_ratio, suggestions))
    matches.sort(key=lambda x: x[0])
    top3 = matches[:3]

    # ── Color sampling ────────────────────────────────────────────────────────
    sample_indices = [0]
    if n_frames > 2:
        sample_indices.append(n_frames // 2)
    if n_frames > 1:
        sample_indices.append(n_frames - 1)
    sample_indices = sorted(set(sample_indices))

    color_lists = []
    for idx in sample_indices:
        img.seek(idx)
        color_lists.append(quantize_frame(img.convert('RGBA'), n_colors=8))
    img.seek(0)

    agg_colors = aggregate_colors(color_lists, n_final=8)
    bg_color = agg_colors[0][0] if agg_colors else "#000000"
    bg_pct = agg_colors[0][1] * 100 if agg_colors else 0

    complexity = color_complexity(agg_colors)
    suggested_colors, complexity_label = suggest_color_count(complexity)

    # ── Output ────────────────────────────────────────────────────────────────
    if args.json:
        import json as _json
        output = {
            "path": os.path.abspath(gif_path),
            "width": width,
            "height": height,
            "frames": n_frames,
            "fps": fps,
            "aspect": round(aspect, 3),
            "bg_color": bg_color,
            "bg_pct": round(bg_pct, 1),
            "suggested_colors": suggested_colors,
            "complexity": round(complexity, 2),
            "complexity_label": complexity_label,
            "widget_sizes": [
                {
                    "size": f"{W}x{H}",
                    "delta": round(delta * 100, 1),
                    "options": [
                        {"cols": c, "rows": r, "font_px": fs}
                        for c, r, fs in suggestions
                    ],
                }
                for delta, W, H, pxW, pxH, widget_ratio, suggestions in top3
            ],
        }
        print(_json.dumps(output))
        return

    print(f"GIF: {os.path.abspath(gif_path)}")
    print(f"Dimensions: {width}x{height} px")
    print(f"Frames: {n_frames}")
    print(f"FPS: {fps} (median frame duration: {median_dur:.0f}ms, uniform: {'yes' if uniform else 'no'})")
    if not uniform:
        print(f"  Note: frame durations vary {min(durations):.0f}–{max(durations):.0f}ms — OMNI uses a single FPS value")
    print(f"Aspect ratio: {aspect:.3f}")
    print()

    print("Widget size matches (by aspect ratio):")
    for delta, W, H, pxW, pxH, widget_ratio, suggestions in top3:
        size_str = f"{W}x{H}"
        opts = "  |  ".join(f"{c}×{r} ({fs}px font)" for c, r, fs in suggestions)
        print(f"  {size_str}  pixel_ratio={widget_ratio:.3f}  delta={delta*100:.1f}%")
        print(f"       Options: {opts}")
    print()

    frame_list_str = ", ".join(str(i + 1) for i in sample_indices)
    total_samples = len(sample_indices) * 16 * 16
    print(f"Color sample (frames {frame_list_str} — {total_samples} sampled cells):")
    shown = 0
    for hex_col, freq in agg_colors:
        if shown < 6:
            print(f"  {hex_col}  {freq*100:.1f}%")
            shown += 1
        else:
            remaining = len(agg_colors) - shown
            if remaining > 0:
                print(f"  + {remaining} more minor color(s)")
            break
    print()

    print(f"Background candidate: {bg_color} ({bg_pct:.1f}% of cells)")
    print(f"Color complexity: {complexity:.2f} ({complexity_label} — suggest {suggested_colors} colors)")
    print()


if __name__ == "__main__":
    main()
