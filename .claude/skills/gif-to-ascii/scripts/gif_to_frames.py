#!/usr/bin/env python3
"""
gif_to_frames.py — Convert a GIF to an OMNI-importable ASCII animation folder.

Uses ascii-image-converter for character mapping and Pillow for per-cell
color extraction. Output uses RLE span grouping and background unspanning
for efficient storage and rendering.

Usage:
  gif_to_frames.py <gif_path>
    --cols <N> --rows <N>
    --colors <N>
    --fps <N>
    --name <animation-name>
    --out <output-dir>
    [--bg-color "#rrggbb"]
    [--char-map "chars"]
    [--grayscale]

Exit codes:
  0 — success
  1 — error
"""

import sys
import os
import re
import math
import json
import argparse
import subprocess
import tempfile
import shutil
import statistics

try:
    from PIL import Image
except ImportError:
    print("Error: Pillow is required. Install with: pip3 install Pillow", file=sys.stderr)
    sys.exit(1)

# ── Palette naming ────────────────────────────────────────────────────────────

NAMES_BY_COUNT = {
    2: ["dark", "bright"],
    3: ["dark", "mid", "bright"],
    4: ["dark", "mid", "light", "bright"],
    5: ["shadow", "dark", "mid", "light", "bright"],
    6: ["shadow", "dark", "mid", "light", "bright", "highlight"],
}


def luminance(r, g, b):
    return 0.299 * r + 0.587 * g + 0.114 * b


def assign_class_names(palette_rgbs):
    """
    Assign luminance-based class names to a list of (r,g,b) palette entries.
    Returns list of (class_name, orig_idx, (r,g,b)) sorted darkest→brightest.
    """
    sorted_palette = sorted(enumerate(palette_rgbs), key=lambda x: luminance(*x[1]))
    n = len(sorted_palette)
    base_names = NAMES_BY_COUNT.get(min(n, 6), NAMES_BY_COUNT[6])
    names = base_names + [f"c{i}" for i in range(6, n)]
    result = []
    for rank, (orig_idx, rgb) in enumerate(sorted_palette):
        name = names[rank] if rank < len(names) else f"c{rank}"
        result.append((name, orig_idx, rgb))
    return result


def hex_to_rgb(h):
    h = h.lstrip('#')
    if len(h) == 3:
        h = ''.join(c * 2 for c in h)
    return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)


def rgb_to_hex(r, g, b):
    return f"#{r:02x}{g:02x}{b:02x}"


def color_distance_sq(c1, c2):
    return sum((a - b) ** 2 for a, b in zip(c1, c2))


# ── Frame extraction ──────────────────────────────────────────────────────────

def extract_frames(gif_path, target_fps):
    """
    Extract all frames from the GIF, resampling to target_fps.
    Returns (list_of_PIL_frames, actual_fps, size_WxH).
    """
    img = Image.open(gif_path)
    n_frames = getattr(img, 'n_frames', 1)

    # Read all frames + durations
    frames = []
    durations = []
    for i in range(n_frames):
        img.seek(i)
        frames.append(img.copy().convert('RGBA'))
        durations.append(img.info.get('duration', 100))
    img.seek(0)

    native_fps = 1000 / statistics.median(durations) if durations else 10.0

    # Resample if target differs from native by more than 5%
    if abs(target_fps - native_fps) / native_fps > 0.05 and target_fps > 0:
        resampled = resample_frames(frames, durations, target_fps)
    else:
        resampled = frames
        target_fps = round(native_fps, 1)

    return resampled, target_fps, img.size


def resample_frames(frames, durations_ms, target_fps):
    """
    Resample frames to a uniform target FPS by selecting the frame
    active at each target time tick.
    """
    total_ms = sum(durations_ms)
    interval_ms = 1000.0 / target_fps
    n_out = max(1, round(total_ms / interval_ms))

    # Build cumulative time map
    cumulative = []
    t = 0
    for d in durations_ms:
        cumulative.append(t)
        t += d

    result = []
    for i in range(n_out):
        t_target = i * interval_ms
        # Find the frame active at t_target
        idx = 0
        for j, start in enumerate(cumulative):
            if start <= t_target:
                idx = j
        result.append(frames[idx])
    return result


# ── ASCII + color conversion ──────────────────────────────────────────────────

def frame_to_char_grid(frame_pil, cols, rows, char_map, tmp_dir, frame_idx):
    """
    Convert a PIL frame to a char grid using ascii-image-converter.
    Falls back to Pillow luminance mapping if the tool fails.
    Returns list of strings (one per row), each exactly cols chars.
    """
    tmp_png = os.path.join(tmp_dir, f"frame_{frame_idx:04d}.png")
    frame_pil.convert('RGB').save(tmp_png)

    txt_path = None
    try:
        cmd = [
            "ascii-image-converter", tmp_png,
            "--save-txt", tmp_dir,
            "--width", str(cols),
            "--height", str(rows),
            "--only-save",
        ]
        if char_map:
            cmd += ["--map", char_map]
        subprocess.run(cmd, capture_output=True, text=True, timeout=60, check=False)
        base = os.path.splitext(os.path.basename(tmp_png))[0]
        txt_path = os.path.join(tmp_dir, f"{base}_ascii_art.txt")
        if os.path.isfile(txt_path):
            with open(txt_path, 'r', encoding='utf-8', errors='replace') as f:
                raw_lines = f.read().splitlines()
            char_grid = []
            for i in range(rows):
                line = raw_lines[i] if i < len(raw_lines) else ''
                line = re.sub(r'\033\[[0-9;]*m', '', line)  # strip ANSI
                if len(line) < cols:
                    line = line + ' ' * (cols - len(line))
                else:
                    line = line[:cols]
                char_grid.append(line)
            return char_grid
    except Exception:
        pass

    # Fallback: Pillow luminance mapping
    chars = char_map if char_map else " .',:;clodxkO0KXN"
    small = frame_pil.convert('L').resize((cols, rows), Image.LANCZOS)
    pixels = list(small.getdata())
    char_grid = []
    for row in range(rows):
        line = ""
        for col in range(cols):
            lum = pixels[row * cols + col]
            idx = int(lum / 255 * (len(chars) - 1))
            line += chars[idx]
        char_grid.append(line)
    return char_grid


def get_color_samples(frames_pil, cols, rows):
    """
    Resize all frames to cols×rows and collect all pixel colors.
    Returns list of flat (r,g,b) lists, one per frame.
    """
    all_colors = []
    for frame in frames_pil:
        small = frame.convert('RGB').resize((cols, rows), Image.LANCZOS)
        all_colors.append(list(small.getdata()))
    return all_colors


def global_quantize(all_color_grids, n_colors):
    """
    Quantize all frames' color data together for a consistent palette.
    Returns (palette_rgbs, list_of_pixel_index_lists).
    """
    # Flatten all colors into one big image for consistent quantization
    flat = []
    for grid in all_color_grids:
        flat.extend(grid)

    n = len(flat)
    w = int(math.ceil(math.sqrt(n)))
    h = int(math.ceil(n / w))
    img = Image.new('RGB', (w, h))
    padded = flat + [(0, 0, 0)] * (w * h - n)
    img.putdata(padded)

    quantized = img.quantize(colors=n_colors, method=Image.Quantize.MEDIANCUT)
    palette_raw = quantized.getpalette()
    palette_rgbs = [
        (palette_raw[i * 3], palette_raw[i * 3 + 1], palette_raw[i * 3 + 2])
        for i in range(n_colors)
    ]

    # Split back into per-frame index lists
    all_indices = list(quantized.getdata())[:n]
    per_frame = []
    # Reconstruct per-frame index lists using frame sizes
    sizes = [len(g) for g in all_color_grids]
    pos = 0
    for size in sizes:
        per_frame.append(all_indices[pos:pos + size])
        pos += size

    return palette_rgbs, per_frame


def rle_row(chars, classes, bg_class, cols):
    """
    Build HTML string for one row with RLE span grouping.
    Background cells (bg_class or None) are written as plain chars.
    """
    result = []
    i = 0
    n = min(len(chars), len(classes), cols)
    while i < n:
        cls = classes[i]
        j = i
        while j < n and classes[j] == cls:
            j += 1
        text = chars[i:j]
        if cls == bg_class or cls is None:
            result.append(text)
        else:
            result.append(f'<span class="{cls}">{text}</span>')
        i = j
    # Pad if short
    remaining = cols - n
    if remaining > 0:
        result.append(' ' * remaining)
    return ''.join(result)


# ── Stats ─────────────────────────────────────────────────────────────────────

def frame_stats(frames_html):
    """Compute quick span stats over all frames."""
    total_spans = sum(f.count('<span') for f in frames_html)
    avg_spans = total_spans / len(frames_html) if frames_html else 0
    total_visible = sum(len(re.sub(r'<[^>]+>', '', f).replace('\n', '')) for f in frames_html)
    avg_run = (total_visible / total_spans) if total_spans > 0 else float('inf')
    total_raw = sum(len(f.encode('utf-8')) for f in frames_html)
    return avg_spans, avg_run, total_raw


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Convert a GIF to OMNI ASCII animation.")
    parser.add_argument("gif_path")
    parser.add_argument("--cols", type=int, required=True)
    parser.add_argument("--rows", type=int, required=True)
    parser.add_argument("--colors", type=int, required=True)
    parser.add_argument("--fps", type=float, required=True)
    parser.add_argument("--name", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--bg-color", default=None)
    parser.add_argument("--char-map", default=None,
                        help="Custom character map for ascii-image-converter (omit to use its default)")
    parser.add_argument("--grayscale", action="store_true")
    args = parser.parse_args()

    if not os.path.isfile(args.gif_path):
        print(f"Error: file not found: {args.gif_path}", file=sys.stderr)
        sys.exit(1)

    n_colors = max(2, args.colors)
    size_str = "1x1"  # will be updated from fit info if agent provides it

    # ── Extract frames ────────────────────────────────────────────────────────
    print(f"Extracting frames from {args.gif_path}...")
    frames_pil, actual_fps, (gif_w, gif_h) = extract_frames(args.gif_path, args.fps)
    n_frames = len(frames_pil)
    print(f"  {n_frames} frames at {actual_fps} FPS ({gif_w}×{gif_h} px source)")

    # Determine size string from cols/rows context (best effort)
    # The agent provides --cols and --rows; we derive size from the output folder name
    # or leave as default. The meta.json size is set below.

    with tempfile.TemporaryDirectory() as tmp_dir:
        # ── Convert each frame to char grid ───────────────────────────────────
        print(f"Converting {n_frames} frames to ASCII ({args.cols}×{args.rows})...")
        char_grids = []
        for i, frame in enumerate(frames_pil):
            if (i + 1) % 5 == 0 or i == 0 or i == n_frames - 1:
                print(f"  Frame {i+1}/{n_frames}...", end='\r')
            grid = frame_to_char_grid(frame, args.cols, args.rows, args.char_map, tmp_dir, i)
            char_grids.append(grid)
        print(f"  Converted {n_frames} frames.      ")

        # ── Color processing ──────────────────────────────────────────────────
        if args.grayscale:
            frames_html = []
            for grid in char_grids:
                frames_html.append('\n'.join(grid))
            palette_for_meta = {}
            bg_class = None
            named_palette = []
            print("Grayscale mode — no color palette.")
        else:
            print("Extracting colors...")
            all_color_grids = get_color_samples(frames_pil, args.cols, args.rows)

            print(f"Quantizing to {n_colors} colors (global palette across all frames)...")
            palette_rgbs, per_frame_indices = global_quantize(all_color_grids, n_colors)
            named_palette = assign_class_names(palette_rgbs)

            # Determine background class
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

            # Build idx→class mapping
            idx_to_class = {orig_idx: cls_name for cls_name, orig_idx, _ in named_palette}

            # Build frame HTML strings
            print("Building frame HTML with RLE spans...")
            frames_html = []
            for fi, (grid, indices) in enumerate(zip(char_grids, per_frame_indices)):
                cell_classes = [idx_to_class.get(pi, bg_class) for pi in indices]
                lines = []
                for r in range(args.rows):
                    row_chars = grid[r]
                    row_classes = cell_classes[r * args.cols:(r + 1) * args.cols]
                    lines.append(rle_row(row_chars, row_classes, bg_class, args.cols))
                frames_html.append('\n'.join(lines))

            # Build palette for meta.json (exclude background class)
            palette_for_meta = {}
            for cls_name, orig_idx, rgb in named_palette:
                if cls_name != bg_class:
                    palette_for_meta[cls_name] = rgb_to_hex(*rgb)

    # ── Write output files ────────────────────────────────────────────────────
    os.makedirs(args.out, exist_ok=True)

    # Derive size string from cols/rows (approximate match to standard sizes)
    # Use a placeholder that the agent can correct in meta.json
    import re as _re
    size_match = _re.search(r'(\d+x\d+)', args.out)
    size_str = size_match.group(1) if size_match else "1x1"
    # Default to 1x1 — agent should correct via --out path convention or meta.json edit
    frames_filename = f"frames-{size_str}.json"

    frames_path = os.path.join(args.out, frames_filename)
    with open(frames_path, 'w', encoding='utf-8') as f:
        json.dump(frames_html, f, ensure_ascii=False, indent=2)

    fps_int = max(1, round(actual_fps))
    meta = {
        "name": args.name,
        "variants": [
            {
                "size": size_str,
                "cols": args.cols,
                "rows": args.rows,
                "fps": fps_int,
                "frames_file": frames_filename,
            }
        ],
    }
    if palette_for_meta:
        meta["palette"] = palette_for_meta

    meta_path = os.path.join(args.out, "meta.json")
    with open(meta_path, 'w', encoding='utf-8') as f:
        json.dump(meta, f, indent=2)

    # ── Print summary ─────────────────────────────────────────────────────────
    avg_spans, avg_run, total_raw = frame_stats(frames_html)

    print(f"\nConverted {n_frames} frames ({args.cols}×{args.rows}, "
          f"{n_colors if not args.grayscale else 0} colors, {fps_int} FPS)")
    print(f"Output: {os.path.abspath(args.out)}/")
    print()

    if not args.grayscale and named_palette:
        print("Final palette (written to meta.json):")
        for cls_name, orig_idx, rgb in sorted(named_palette, key=lambda x: luminance(*x[2])):
            hex_col = rgb_to_hex(*rgb)
            bg_note = "  (background — unspanned)" if cls_name == bg_class else ""
            print(f"  {cls_name:<10} → {hex_col}{bg_note}")
        print()

    print("Stats (run validate.py for exact size analysis):")
    print(f"  Avg spans/frame: {avg_spans:.0f}")
    if avg_run == float('inf'):
        print("  Avg run length:  n/a (no spans)")
    else:
        print(f"  Avg run length:  {avg_run:.1f} chars")
    print(f"  Total raw size:  {total_raw / 1024:.1f} KB")
    print()


if __name__ == "__main__":
    main()
