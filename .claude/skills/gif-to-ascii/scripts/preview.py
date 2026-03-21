#!/usr/bin/env python3
"""
preview.py — Play an ICG ASCII animation in the terminal.

Usage:
  preview.py <path-to-animation-dir> [--variant SIZE] [--loop]

Examples:
  preview.py ./ascii-out/spinner
  preview.py ./ascii-out/spinner --variant 1x1
  preview.py ./ascii-out/spinner --loop
  preview.py ./examples/sample-pack/wave --loop

Controls:
  Ctrl+C to stop playback
"""

import argparse
import base64
import json
import os
import re
import sys
import time

# ANSI codes
RESET       = "\033[0m"
HIDE_CURSOR = "\033[?25l"
SHOW_CURSOR = "\033[?25h"
MOVE_HOME   = "\033[H"
CLEAR       = "\033[2J"


# ── CSS color → ANSI 256 approximation ───────────────────────────────────────

def parse_hex(h):
    """Parse hex color string (#RGB or #RRGGBB or #RRGGBBAA) → (r, g, b)."""
    h = h.lstrip('#')
    if len(h) in (3, 4):
        h = ''.join(c*2 for c in h[:3])
    if len(h) >= 6:
        return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)
    return None


def parse_rgb_func(s):
    """Parse rgb(r, g, b) or rgba(r, g, b, a) → (r, g, b)."""
    m = re.match(r'rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)', s)
    if m:
        return int(m.group(1)), int(m.group(2)), int(m.group(3))
    return None


# Named CSS colors (subset)
NAMED_COLORS = {
    'red': (255,0,0), 'green': (0,128,0), 'blue': (0,0,255),
    'white': (255,255,255), 'black': (0,0,0), 'yellow': (255,255,0),
    'cyan': (0,255,255), 'magenta': (255,0,255), 'orange': (255,165,0),
    'pink': (255,192,203), 'purple': (128,0,128), 'lime': (0,255,0),
    'navy': (0,0,128), 'teal': (0,128,128), 'silver': (192,192,192),
    'gray': (128,128,128), 'grey': (128,128,128), 'maroon': (128,0,0),
    'olive': (128,128,0), 'aqua': (0,255,255), 'fuchsia': (255,0,255),
    'coral': (255,127,80), 'gold': (255,215,0), 'indigo': (75,0,130),
    'violet': (238,130,238), 'salmon': (250,128,114), 'tan': (210,180,140),
    'turquoise': (64,224,208), 'khaki': (240,230,140),
}


def css_to_rgb(css):
    """Convert a CSS color string to (r, g, b) tuple, or None."""
    css = css.strip()
    if css.startswith('#'):
        return parse_hex(css)
    if css.lower().startswith('rgb'):
        return parse_rgb_func(css)
    return NAMED_COLORS.get(css.lower())


def rgb_to_ansi256(r, g, b):
    """Convert RGB to nearest ANSI 256 color index."""
    # Check grayscale ramp (232–255): 24 shades
    gray = (r == g == b)
    if gray:
        if r < 8:
            return 16
        if r > 248:
            return 231
        return round((r - 8) / 247 * 23) + 232

    # 6×6×6 color cube (16–231)
    ri = round(r / 255 * 5)
    gi = round(g / 255 * 5)
    bi = round(b / 255 * 5)
    return 16 + 36 * ri + 6 * gi + bi


def ansi_fg(r, g, b):
    return f"\033[38;5;{rgb_to_ansi256(r, g, b)}m"


# ── ICG Frame renderer ──────────────────────────────────────────────────────

def render_frame_icg(frame, class_table, class_ansi):
    """
    Render an ICG frame to a terminal string with ANSI colors.

    frame:       dict with "chars" (plain text, \\n-separated) and "colors" (base64)
    class_table: list of class names, index 0 = "" = default color
    class_ansi:  dict of class_name → ANSI escape string (precomputed)
    """
    chars = frame["chars"]
    color_bytes = base64.b64decode(frame["colors"])

    output = []
    cell_idx = 0
    prev_ansi = None

    for ch in chars:
        if ch == '\n':
            # Reset color at end of row, then newline
            if prev_ansi is not None:
                output.append(RESET)
                prev_ansi = None
            output.append('\n')
            continue

        # Look up color for this cell
        if cell_idx < len(color_bytes):
            class_idx = color_bytes[cell_idx]
        else:
            class_idx = 0

        class_name = class_table[class_idx] if class_idx < len(class_table) else ""
        ansi = class_ansi.get(class_name) if class_name else None

        # Only emit ANSI escape when the color changes
        if ansi != prev_ansi:
            if ansi is None:
                output.append(RESET)
            else:
                output.append(ansi)
            prev_ansi = ansi

        output.append(ch)
        cell_idx += 1

    # Final reset
    if prev_ansi is not None:
        output.append(RESET)

    return ''.join(output)


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Preview an ICG ASCII animation in the terminal.")
    parser.add_argument("path", help="Path to animation directory")
    parser.add_argument("--variant", default=None, help="Size variant to play, e.g. 1x1 (default: first)")
    parser.add_argument("--loop", action="store_true", help="Loop indefinitely until Ctrl+C")
    args = parser.parse_args()

    anim_dir = os.path.abspath(args.path)
    meta_path = os.path.join(anim_dir, "meta.json")

    if not os.path.isfile(meta_path):
        print(f"Error: meta.json not found in {anim_dir}", file=sys.stderr)
        sys.exit(1)

    with open(meta_path, 'r', encoding='utf-8') as f:
        meta = json.load(f)

    variants = meta.get("variants", [])
    if not variants:
        print("Error: no variants in meta.json", file=sys.stderr)
        sys.exit(1)

    # Select variant
    variant = variants[0]
    if args.variant:
        for v in variants:
            if v.get("size", "").lower() == args.variant.lower():
                variant = v
                break
        else:
            print(f"Warning: variant '{args.variant}' not found, using first variant", file=sys.stderr)

    size = variant.get("size", "?")
    cols = variant.get("cols", 40)
    rows = variant.get("rows", 20)
    fps  = max(1, variant.get("fps", 8))
    frames_file = variant.get("frames_file", "")

    frames_path = os.path.join(anim_dir, frames_file)
    if not os.path.isfile(frames_path):
        print(f"Error: frames file '{frames_file}' not found", file=sys.stderr)
        sys.exit(1)

    with open(frames_path, 'r', encoding='utf-8') as f:
        frames_data = json.load(f)

    # ICG format: { "class_table": [...], "frames": [{ "chars": "...", "colors": "..." }, ...] }
    class_table = frames_data.get("class_table", [""])
    frames = frames_data.get("frames", [])

    if not frames:
        print("Error: frames array is empty", file=sys.stderr)
        sys.exit(1)

    # Build ANSI palette from class names
    palette = meta.get("palette") or {}
    class_ansi = {}
    supports_color = sys.stdout.isatty()
    if supports_color:
        for cls_name in class_table:
            if not cls_name:
                continue  # index 0 = default, no color override
            css_color = palette.get(cls_name)
            if css_color:
                rgb = css_to_rgb(css_color)
                if rgb:
                    class_ansi[cls_name] = ansi_fg(*rgb)

    delay = 1.0 / fps
    name = meta.get("name", "?")

    print(f"\nPlaying '{name}' variant {size}  ({cols}x{rows}, {fps} FPS, {len(frames)} frames)")
    if args.loop:
        print("Looping — press Ctrl+C to stop\n")
    else:
        print()

    if supports_color:
        sys.stdout.write(HIDE_CURSOR)
        sys.stdout.flush()

    try:
        iteration = 0
        while True:
            for fi, frame in enumerate(frames):
                rendered = render_frame_icg(frame, class_table, class_ansi)
                lines = rendered.split('\n')

                if supports_color:
                    sys.stdout.write(MOVE_HOME)
                    sys.stdout.write('\n'.join(lines))
                    sys.stdout.flush()
                else:
                    # No color — print separator
                    print(f"\n--- frame {fi+1}/{len(frames)} ---")
                    print('\n'.join(lines))

                time.sleep(delay)

            iteration += 1
            if not args.loop:
                break

    except KeyboardInterrupt:
        pass
    finally:
        if supports_color:
            sys.stdout.write(SHOW_CURSOR)
            sys.stdout.write('\n')
            sys.stdout.flush()

    if not supports_color or not args.loop:
        print(f"\nDone. ({len(frames)} frames played)")


if __name__ == "__main__":
    main()
