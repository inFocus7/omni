#!/usr/bin/env python3
"""
estimate.py — Pre-conversion size estimate for a GIF-to-ASCII conversion.

Converts a single sample frame using the provided settings, measures the
resulting ICG frame data, and extrapolates to the full animation size.

Usage:
  estimate.py <gif_path>
    --cols <N> --rows <N>
    --colors <N>
    --fps <N>
    [--bg-color "#rrggbb"]
    [--char-map "chars"]
    [--frame first|random|<N>]   default: first
    [--grayscale]

Exit codes: 0 always (advisory output only, never hard-fails)
"""

import argparse
import base64
import math
import os
import random
import re
import subprocess
import sys
import tempfile

try:
    from PIL import Image
except ImportError:
    print("Error: Pillow is required. Install with: pip3 install Pillow", file=sys.stderr)
    sys.exit(1)

# Import shared metrics + thresholds from validate.py (same scripts/ dir)
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from validate import apply_thresholds, compute_icg_metrics

# ── Color helpers ─────────────────────────────────────────────────────────────

def rgb_to_hex(r, g, b):
    return f"#{r:02x}{g:02x}{b:02x}"


def color_distance_sq(c1, c2):
    return sum((a - b) ** 2 for a, b in zip(c1, c2))


def hex_to_rgb(h):
    h = h.lstrip('#')
    if len(h) == 3:
        h = ''.join(c * 2 for c in h)
    return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)


def luminance(r, g, b):
    return 0.299 * r + 0.587 * g + 0.114 * b


# ── Palette naming (shared with gif_to_frames.py) ────────────────────────────

NAMES_BY_COUNT = {
    2: ["dark", "bright"],
    3: ["dark", "mid", "bright"],
    4: ["dark", "mid", "light", "bright"],
    5: ["shadow", "dark", "mid", "light", "bright"],
    6: ["shadow", "dark", "mid", "light", "bright", "highlight"],
}


def assign_class_names(palette_rgbs):
    """
    Given a list of (r,g,b) tuples, assign luminance-based class names.
    Returns list of (class_name, orig_idx, (r,g,b)) sorted darkest→brightest.
    """
    sorted_palette = sorted(enumerate(palette_rgbs), key=lambda x: luminance(*x[1]))
    n = len(sorted_palette)
    names = NAMES_BY_COUNT.get(n, None)
    if names is None:
        base = NAMES_BY_COUNT.get(6, [f"c{i}" for i in range(6)])
        names = base + [f"c{i}" for i in range(6, n)]
    result = []
    for rank, (orig_idx, rgb) in enumerate(sorted_palette):
        name = names[rank] if rank < len(names) else f"c{rank}"
        result.append((name, orig_idx, rgb))
    return result  # (class_name, original_palette_index, (r,g,b))


# ── ASCII conversion (single frame) ──────────────────────────────────────────

def frame_to_char_grid(frame_img, cols, rows, char_map, tmp_dir):
    """
    Use ascii-image-converter to convert a PIL Image frame to a char grid.
    Returns list of strings (one per row), each exactly `cols` chars wide.
    Falls back to luminance mapping if ascii-image-converter fails.
    """
    tmp_png = os.path.join(tmp_dir, "frame_estimate.png")
    frame_img.convert('RGB').save(tmp_png)

    # Try ascii-image-converter
    try:
        cmd = [
            "ascii-image-converter", tmp_png,
            "--save-txt", tmp_dir,
            "--width", str(cols),
            "--height", str(rows),
        ]
        if char_map:
            cmd += ["--map", char_map]
        subprocess.run(cmd, capture_output=True, text=True, timeout=30, check=False)
        # ascii-image-converter saves to <basename>_ascii_art.txt
        base = os.path.splitext(os.path.basename(tmp_png))[0]
        txt_path = os.path.join(tmp_dir, f"{base}_ascii_art.txt")
        if os.path.isfile(txt_path):
            with open(txt_path, 'r', encoding='utf-8', errors='replace') as f:
                lines = f.read().splitlines()
            # Pad/trim to exactly cols×rows
            char_grid = []
            for i in range(rows):
                line = lines[i] if i < len(lines) else ''
                # Strip ANSI codes if any
                line = re.sub(r'\033\[[0-9;]*m', '', line)
                if len(line) < cols:
                    line = line + ' ' * (cols - len(line))
                else:
                    line = line[:cols]
                char_grid.append(line)
            return char_grid
    except Exception as e:
        print(f"Warning: ascii-image-converter failed: {e}", file=sys.stderr)

    # Fallback: luminance-based char mapping using Pillow (lower quality)
    chars = char_map if char_map else " .',:;clodxkO0KXN"
    small = frame_img.convert('L').resize((cols, rows), Image.Resampling.LANCZOS)
    char_grid = []
    pixels = list(small.getdata())
    for row in range(rows):
        line = ""
        for col in range(cols):
            lum = pixels[row * cols + col]
            idx = int(lum / 255 * (len(chars) - 1))
            line += chars[idx]
        char_grid.append(line)
    return char_grid


def get_color_grid(frame_img, cols, rows):
    """
    Resize frame to cols×rows and return flat list of (r,g,b) per cell.
    """
    small = frame_img.convert('RGB').resize((cols, rows), Image.Resampling.LANCZOS)
    return list(small.getdata())


def quantize_colors(color_grid, n_colors):
    """
    Quantize the color grid to n_colors using a small PIL image.
    Returns (palette_rgbs, pixel_indices) where palette_rgbs is list of (r,g,b)
    and pixel_indices maps each cell to a palette index.
    """
    # Build tiny image from color_grid
    n = len(color_grid)
    w = int(math.ceil(math.sqrt(n)))
    h = int(math.ceil(n / w))
    img = Image.new('RGB', (w, h))
    padded = list(color_grid) + [(0, 0, 0)] * (w * h - n)
    img.putdata(padded)
    quantized = img.quantize(colors=n_colors, method=Image.Quantize.MEDIANCUT)
    palette_raw = quantized.getpalette()
    if not palette_raw:
        raise ValueError("Quantization failed: no palette found")
    palette_rgbs = [
        (palette_raw[i * 3], palette_raw[i * 3 + 1], palette_raw[i * 3 + 2])
        for i in range(n_colors)
    ]
    indices = list(quantized.getdata())[:n]
    return palette_rgbs, indices


def build_icg_frame(char_grid, cell_classes, class_to_idx, cols, rows):
    """
    Build an ICG frame: { chars: str, colors: base64-encoded bytes }.
    char_grid is list of row strings, cell_classes is a flat list of class names.
    """
    lines = []
    colors = bytearray(cols * rows)
    for r in range(rows):
        row_chars = char_grid[r] if r < len(char_grid) else ' ' * cols
        if len(row_chars) < cols:
            row_chars = row_chars + ' ' * (cols - len(row_chars))
        elif len(row_chars) > cols:
            row_chars = row_chars[:cols]
        lines.append(row_chars)
        for c in range(cols):
            cls = cell_classes[r * cols + c] if r * cols + c < len(cell_classes) else ""
            colors[r * cols + c] = class_to_idx.get(cls, 0)
    return {
        "chars": '\n'.join(lines),
        "colors": base64.b64encode(bytes(colors)).decode('ascii'),
    }


# ── Output formatting ─────────────────────────────────────────────────────────

def format_estimate(metrics, frame_label, cols, rows, n_frames):
    lines = []
    lines.append(f"\nSize Estimate — based on {frame_label} of {n_frames}")
    lines.append("-" * 66)
    lines.append(f"Grid:              {cols} x {rows} = {cols*rows} chars/frame")
    lines.append(f"class_table:       {metrics['class_count']} entries")
    lines.append(f"Avg chars/frame:   {metrics['avg_chars_kb']:.2f} KB")
    lines.append(f"Avg colors/frame:  {metrics['avg_colors_kb']:.2f} KB")
    lines.append(
        f"Default ratio:     {metrics['default_ratio']*100:.0f}%"
        f"  (cells with class_table[0])"
    )
    lines.append("")
    lines.append(f"Frame size (raw):  {metrics['raw_kb'] / n_frames:.1f} KB")
    lines.append(f"Total raw ({n_frames}f):   ~{metrics['raw_kb']:.1f} KB")
    if metrics['gz_range']:
        lo, hi = metrics['gz_range']
        lines.append(f"Total gzip (est):  ~{lo:.1f}-{hi:.1f} KB  (20-40% heuristic)")
        lines.append("  Note: actual gzip may be much smaller -- inter-frame repetition compresses")
        lines.append("  far better than this single-frame estimate can predict.")
    lines.append("")
    lines.append("Recommendations:")
    recs = apply_thresholds(metrics)
    if not recs:
        lines.append("  OK All size metrics look healthy")
        lines.append(f"  OK Default ratio is good ({metrics['default_ratio']*100:.0f}%)")
        lines.append(f"  OK Frame count is reasonable ({n_frames} frames)")
    else:
        healthy = []
        if not any('class_table' in m.lower() or 'palette' in m.lower() for _, m in recs):
            healthy.append(f"OK class_table size is healthy ({metrics['class_count']} entries)")
        if not any('frame' in m.lower() for _, m in recs):
            healthy.append(f"OK Frame count is reasonable ({n_frames} frames)")
        for h in healthy:
            lines.append(f"  {h}")
        for sev, msg in recs:
            sym = "!!" if sev == 2 else "! "
            lines.append(f"  {sym} {msg}")
    lines.append("")
    lines.append(f"Note: estimate from {frame_label}. Complex frames may be larger; simple frames smaller.")
    return '\n'.join(lines)


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Pre-conversion size estimate for GIF-to-ASCII.")
    parser.add_argument("gif_path")
    parser.add_argument("--cols", type=int, required=True)
    parser.add_argument("--rows", type=int, required=True)
    parser.add_argument("--colors", type=int, required=True)
    parser.add_argument("--fps", type=float, required=True)
    parser.add_argument("--bg-color", default=None)
    parser.add_argument("--char-map", default=None,
                        help="Custom character map for ascii-image-converter (omit to use its default)")
    parser.add_argument("--frame", default="first",
                        help="first | random | <N>  (0-indexed frame number)")
    parser.add_argument("--grayscale", action="store_true")
    args = parser.parse_args()

    if not os.path.isfile(args.gif_path):
        print(f"Error: file not found: {args.gif_path}", file=sys.stderr)
        sys.exit(0)

    try:
        img = Image.open(args.gif_path)
    except Exception as e:
        print(f"Error: cannot open GIF: {e}", file=sys.stderr)
        sys.exit(0)

    n_frames = getattr(img, 'n_frames', 1)

    # Select frame
    if args.frame == "first":
        frame_idx = 0
        frame_label = "frame 1"
    elif args.frame == "random":
        frame_idx = random.randint(0, n_frames - 1)
        frame_label = f"frame {frame_idx + 1} (random)"
    else:
        try:
            frame_idx = max(0, min(int(args.frame), n_frames - 1))
            frame_label = f"frame {frame_idx + 1}"
        except ValueError:
            frame_idx = 0
            frame_label = "frame 1"

    img.seek(frame_idx)
    frame_pil = img.copy().convert('RGBA')
    img.seek(0)

    with tempfile.TemporaryDirectory() as tmp_dir:
        # 1. Get char grid
        char_grid = frame_to_char_grid(frame_pil, args.cols, args.rows, args.char_map, tmp_dir)

        if args.grayscale:
            # Grayscale: no colors, all index 0 = default
            class_table = [""]
            icg_frame = build_icg_frame(
                char_grid, [""] * (args.cols * args.rows), {"": 0}, args.cols, args.rows
            )
            icg_data = {"class_table": class_table, "frames": [icg_frame]}
        else:
            # 2. Get color grid
            color_grid = get_color_grid(frame_pil, args.cols, args.rows)

            # 3. Quantize to N colors
            n_colors = max(2, args.colors)
            palette_rgbs, pixel_indices = quantize_colors(color_grid, n_colors)

            # 4. Assign class names by luminance
            named_palette = assign_class_names(palette_rgbs)

            # 5. Determine background class
            bg_class = None
            if args.bg_color:
                bg_rgb = hex_to_rgb(args.bg_color)
                best_cls = None
                best_d = float('inf')
                for cls_name, orig_idx, pal_rgb in named_palette:
                    d = color_distance_sq(bg_rgb, pal_rgb)
                    if d < best_d:
                        best_d = d
                        best_cls = cls_name
                bg_class = best_cls

            # 6. Build class_table and mapping
            idx_to_class = {orig_idx: cls_name for cls_name, orig_idx, _ in named_palette}
            class_table = [""]
            class_to_idx = {"": 0}
            for cls_name, orig_idx, _ in named_palette:
                if cls_name != bg_class:
                    class_to_idx[cls_name] = len(class_table)
                    class_table.append(cls_name)
            if bg_class:
                class_to_idx[bg_class] = 0

            # 7. Map cells to class names
            cell_classes = [idx_to_class.get(pi, "") for pi in pixel_indices]
            cell_classes = ["" if c == bg_class else c for c in cell_classes]

            # 8. Build ICG frame
            icg_frame = build_icg_frame(char_grid, cell_classes, class_to_idx, args.cols, args.rows)
            icg_data = {"class_table": class_table, "frames": [icg_frame]}

        # Compute metrics using the ICG data
        metrics = compute_icg_metrics(icg_data, fps=args.fps)

        # Extrapolate to full animation
        metrics['raw_kb'] = metrics['raw_kb'] * n_frames
        lo = metrics['raw_kb'] * 0.20
        hi = metrics['raw_kb'] * 0.40
        metrics['gz_range'] = (lo, hi)
        metrics['gz_kb'] = None
        metrics['frame_count'] = n_frames

    print()
    print(format_estimate(metrics, frame_label, args.cols, args.rows, n_frames))
    print()


if __name__ == "__main__":
    main()
