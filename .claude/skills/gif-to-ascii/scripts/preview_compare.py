#!/usr/bin/env python3
"""
preview_compare.py — Single-frame preview comparisons for OMNI gif-to-ascii.

Always samples exactly ONE frame from the GIF (--frame, default 0).
No full animation is rendered — intentionally fast and lightweight.

Intended use: the agent runs this to generate visual options, then presents
them to the user as clickable choices before running gif_to_frames.py.

Three modes, meant to be run in order:

  sizes      Compare cols x rows candidates — run FIRST
  charsets   Compare character sets at a confirmed size — run SECOND
  palettes   Compare color palettes at confirmed size+charset — run LAST

Sizes mode requires either --sizes-json (JSON output from gif_info.py --json,
which derives candidates from the GIF aspect ratio) or an explicit --sizes list.
No hardcoded size defaults.

Charset and palette lists are entirely agent-supplied via --charsets and
--palettes. No hardcoded defaults. The agent chooses which options to show
based on context and user preferences. Built-in named presets are documented
in this docstring for the agent to reference when constructing arguments.

Usage:
  # Step 1 — compare sizes
  python3 preview_compare.py <gif> --mode sizes
      --sizes-json '<json from gif_info.py --json>'
      [--chars "..."]  [--frame N]

  # Step 1 alt — explicit size list
  python3 preview_compare.py <gif> --mode sizes
      --sizes "50x24,74x36,80x39"
      [--chars "..."]  [--frame N]

  # Step 2 — compare charsets (size confirmed)
  python3 preview_compare.py <gif> --mode charsets
      --cols N --rows N
      --charsets " .+#@| .:;=+xX#@| .',:;clodxkO0KXN@"
      [--frame N]

  # Step 3 — compare palettes (size + charset confirmed)
  python3 preview_compare.py <gif> --mode palettes
      --cols N --rows N --chars "..."
      --palettes "white:#888888,#ffffff|grey:#aaaaaa,#dddddd"
      [--bg-color "#hex"]  [--colors N]  [--frame N]

Output:
  Text blocks the agent embeds in a visualizer widget, followed by a
  JSON_SUMMARY block the agent parses to extract the user's chosen settings.

---
AGENT REFERENCE — charset presets:

  default      " .',:;clodxkO0KXN@"   general purpose, broadest tonal range
  detailed     " .:;=+xX$&#@"         good density steps, slightly noisy
  clean        " .,:;=+xX#@"          similar to detailed, slightly less busy
  blocks       " ░▒▓█"                bold, minimal, retro terminal
  minimal      " .+#@"                very sparse — loses fine detail
  dots         " .·:+%#█"             mixed dot/block, mid-weight
  punctuation  " .,;:!|/()[]@#"       dense, high-noise texture

  Avoid asymmetric chars like {}[]() — they disrupt halftone texture.
  Chars that read as density: . , : ; = + x X # @ % ░ ▒ ▓ █

AGENT REFERENCE — palette presets (neutral first, accents after):

  auto         auto-detected from GIF (always shown as first option)
  white        #888888, #ffffff
  light-grey   #aaaaaa, #dddddd
  warm-white   #998870, #fff5e0
  cool-grey    #778899, #ccdde8
  green        #00aa2a, #00ff41
  amber        #aa6600, #ffcc00
  cyan         #007799, #00ccff
  red          #aa1100, #ff3300
  purple       #550088, #cc00ff

  Pass as: "white:#888888,#ffffff|light-grey:#aaaaaa,#dddddd"
  Colors ordered darkest to brightest, background excluded.
"""

import argparse
import json
import os
import sys

try:
    from PIL import Image
except ImportError:
    print("Error: Pillow is required. pip3 install Pillow", file=sys.stderr)
    sys.exit(1)


# ── Helpers ───────────────────────────────────────────────────────────────────

def luminance(r, g, b):
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def hex_to_rgb(h):
    h = h.lstrip('#')
    if len(h) == 3:
        h = ''.join(c * 2 for c in h)
    return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)


def rgb_to_hex(r, g, b):
    return f"#{r:02x}{g:02x}{b:02x}"


def color_distance_sq(c1, c2):
    return sum((a - b) ** 2 for a, b in zip(c1, c2))


def extract_frame(gif_path, frame_idx=0):
    """Extract exactly one frame. Never renders the full animation."""
    img = Image.open(gif_path)
    n_frames = getattr(img, 'n_frames', 1)
    idx = min(frame_idx, n_frames - 1)
    img.seek(idx)
    return img.copy().convert('RGBA'), n_frames, img.size


def frame_to_chars(frame_pil, cols, rows, chars):
    """Luminance -> character mapping. Returns list of row strings."""
    small = frame_pil.convert('L').resize((cols, rows), Image.Resampling.LANCZOS)
    pixels = list(small.getdata())
    lines = []
    for r in range(rows):
        line = ""
        for c in range(cols):
            lum = pixels[r * cols + c]
            idx = int(lum / 255 * (len(chars) - 1))
            line += chars[idx]
        lines.append(line)
    return lines


def quantize_frame(frame_pil, cols, rows, n_colors, bg_rgb=None):
    """
    Quantize one frame to n_colors.
    Returns (indices, palette_rgbs, bg_idx, bg_hex, non_bg_palette).
    non_bg_palette: list of {class, hex, orig_idx} sorted darkest->brightest.
    """
    from collections import Counter

    NAMES = {
        2: ["dark", "bright"],
        3: ["dark", "mid", "bright"],
        4: ["dark", "mid", "light", "bright"],
        5: ["shadow", "dark", "mid", "light", "bright"],
        6: ["shadow", "dark", "mid", "light", "bright", "highlight"],
    }

    small = frame_pil.convert('RGB').resize((cols, rows), Image.Resampling.LANCZOS)
    pixels = list(small.getdata())
    img_q = Image.new('RGB', (cols, rows))
    img_q.putdata(pixels)
    quantized = img_q.quantize(colors=n_colors, method=Image.Quantize.MEDIANCUT)
    pal_raw = quantized.getpalette()
    if pal_raw is None:
        raise ValueError("Quantization failed to produce a palette")
    pal_rgbs = [
        (pal_raw[i*3], pal_raw[i*3+1], pal_raw[i*3+2])
        for i in range(n_colors)
    ]
    indices = list(quantized.getdata())

    if bg_rgb:
        bg_idx = min(range(n_colors),
                     key=lambda i: color_distance_sq(pal_rgbs[i], bg_rgb))
    else:
        bg_idx = Counter(indices).most_common(1)[0][0]

    bg_hex = rgb_to_hex(*pal_rgbs[bg_idx])
    non_bg = sorted(
        [(i, rgb) for i, rgb in enumerate(pal_rgbs) if i != bg_idx],
        key=lambda x: luminance(*x[1])
    )
    n = len(non_bg)
    base = NAMES.get(min(n, 6), NAMES[6])
    names = base + [f"c{i}" for i in range(6, n)]

    palette_out = [
        {"class": names[rank], "hex": rgb_to_hex(*rgb), "orig_idx": orig_idx}
        for rank, (orig_idx, rgb) in enumerate(non_bg)
    ]
    return indices, pal_rgbs, bg_idx, bg_hex, palette_out


# ── Mode implementations ──────────────────────────────────────────────────────

def mode_sizes(gif_path, sizes, chars, frame_idx):
    """
    sizes: list of (label, cols, rows) — derived from gif_info JSON or explicit.
    Renders each at the given chars. Frame is sampled once and reused.
    """
    frame, n_frames, src_size = extract_frame(gif_path, frame_idx)
    results = []
    for label, cols, rows in sizes:
        grid = frame_to_chars(frame, cols, rows, chars)
        results.append({
            "label": label, "chars": chars,
            "cols": cols, "rows": rows, "lines": grid,
        })
    return results, n_frames, src_size


def mode_charsets(gif_path, cols, rows, charsets, frame_idx):
    """
    charsets: list of (label, chars) — agent-supplied, no defaults.
    Frame is sampled once and reused across all charset renders.
    """
    frame, n_frames, src_size = extract_frame(gif_path, frame_idx)
    results = []
    for label, chars in charsets:
        grid = frame_to_chars(frame, cols, rows, chars)
        results.append({
            "label": label, "chars": chars,
            "cols": cols, "rows": rows, "lines": grid,
        })
    return results, n_frames, src_size


def mode_palettes(gif_path, cols, rows, chars, palette_overrides,
                  frame_idx, n_colors, bg_color):
    """
    palette_overrides: list of (label, [hex, ...]) — agent-supplied.
    Always prepends 'auto-detected' as the first option.
    Char grid is rendered ONCE and reused — palette mode is very fast.
    """
    frame, n_frames, src_size = extract_frame(gif_path, frame_idx)
    bg_rgb = hex_to_rgb(bg_color) if bg_color else None

    _, pal_rgbs, bg_idx, bg_hex, auto_palette = quantize_frame(
        frame, cols, rows, n_colors, bg_rgb
    )
    base_grid = frame_to_chars(frame, cols, rows, chars)

    results = [{
        "label": "auto-detected",
        "palette": [p["hex"] for p in auto_palette],
        "bg_hex": bg_hex,
        "chars": chars, "cols": cols, "rows": rows,
        "lines": base_grid,
        "palette_display": auto_palette,
    }]

    for label, override_hexes in palette_overrides:
        results.append({
            "label": label,
            "palette": override_hexes,
            "bg_hex": bg_hex,
            "chars": chars, "cols": cols, "rows": rows,
            "lines": base_grid,
            "palette_display": [
                {"class": f"c{i}", "hex": h}
                for i, h in enumerate(override_hexes)
            ],
        })

    return results, n_frames, src_size, bg_hex, auto_palette


# ── Output formatting ─────────────────────────────────────────────────────────

def print_results(results, mode, n_frames, src_size):
    print(f"SOURCE: {src_size[0]}x{src_size[1]}px, {n_frames} frames")
    print(f"MODE: {mode}")
    print(f"COMPARISONS: {len(results)}")
    print()

    for r in results:
        print(f"=== {r['label']} ===")
        if mode == "charsets":
            print(f"chars: {repr(r['chars'])}")
        elif mode == "sizes":
            print(f"size: {r['cols']}x{r['rows']}  chars: {repr(r['chars'])}")
        elif mode == "palettes":
            pal_str = "  ".join(f"{p['class']}={p['hex']}" for p in r['palette_display'])
            print(f"palette: {pal_str}")
        print()
        for line in r['lines']:
            print(line)
        print()

    summary = []
    for r in results:
        entry = {"label": r["label"], "cols": r["cols"], "rows": r["rows"]}
        if mode == "charsets":
            entry["chars"] = r["chars"]
        elif mode == "palettes":
            entry["palette"] = r["palette"]
            entry["bg"] = r.get("bg_hex")
        summary.append(entry)

    print("JSON_SUMMARY:")
    print(json.dumps(summary, indent=2))


# ── Argument parsing helpers ──────────────────────────────────────────────────

def parse_sizes_arg(s):
    """'50x24,74x36' -> [('50x24', 50, 24), ...]"""
    results = []
    for item in s.split(','):
        item = item.strip()
        if 'x' in item.lower():
            parts = item.lower().split('x')
            try:
                results.append((item, int(parts[0]), int(parts[1])))
            except ValueError:
                pass
    return results


def parse_sizes_from_gif_info_json(json_str):
    """
    Parse JSON output from gif_info.py --json.
    Expects a 'widget_sizes' list, each entry with 'size' and 'options'
    (each option has 'cols', 'rows', 'font_px').
    Returns list of (label, cols, rows).
    """
    try:
        data = json.loads(json_str)
        sizes = []
        for ws in data.get("widget_sizes", []):
            for opt in ws.get("options", []):
                label = f"{ws['size']}  {opt['cols']}x{opt['rows']} ({opt['font_px']}px)"
                sizes.append((label, opt["cols"], opt["rows"]))
        return sizes
    except Exception as e:
        print(f"Error parsing --sizes-json: {e}", file=sys.stderr)
        sys.exit(1)


def parse_charsets_arg(s):
    """Pipe-separated charset strings -> [(label, chars), ...]"""
    results = []
    for i, c in enumerate(s.split('|')):
        c = c.strip()
        if c:
            results.append((f"option-{i+1}", c))
    return results


def parse_palettes_arg(s):
    """'label:hex1,hex2|label2:hex1,hex2' -> [(label, [hex,...]), ...]"""
    results = []
    for group in s.split('|'):
        group = group.strip()
        if ':' in group:
            label, hexes_str = group.split(':', 1)
            hexes = [h.strip() for h in hexes_str.split(',') if h.strip()]
            if hexes:
                results.append((label.strip(), hexes))
    return results


# ── CLI ───────────────────────────────────────────────────────────────────────

def main():
    p = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    p.add_argument("gif_path")
    p.add_argument("--mode", choices=["sizes", "charsets", "palettes"],
                   required=True,
                   help="Comparison mode. Run in order: sizes -> charsets -> palettes")
    p.add_argument("--frame", type=int, default=0,
                   help="Frame index to sample, 0-based (default: 0). Always one frame.")

    # sizes mode
    p.add_argument("--sizes", default=None,
                   help="Explicit size list: '50x24,74x36,80x39'")
    p.add_argument("--sizes-json", default=None,
                   help="JSON string from gif_info.py --json (derives size options from GIF aspect ratio)")

    # charsets mode
    p.add_argument("--cols", type=int, default=None,
                   help="Confirmed cols (required for charsets and palettes modes)")
    p.add_argument("--rows", type=int, default=None,
                   help="Confirmed rows (required for charsets and palettes modes)")
    p.add_argument("--charsets", default=None,
                   help="Pipe-separated charset strings to compare: ' .+#@| .:;=#@'")

    # palettes mode
    p.add_argument("--chars", default=" .',:;clodxkO0KXN@",
                   help="Confirmed character map (used for palettes and sizes modes)")
    p.add_argument("--palettes", default=None,
                   help="Pipe-separated palette overrides: 'white:#888888,#ffffff|grey:#aaa,#ddd'")
    p.add_argument("--bg-color", default=None,
                   help="Background hex color e.g. '#000000' (palettes mode)")
    p.add_argument("--colors", type=int, default=4,
                   help="Number of palette colors to quantize (palettes mode, default 4)")

    args = p.parse_args()

    if not os.path.isfile(args.gif_path):
        print(f"Error: file not found: {args.gif_path}", file=sys.stderr)
        sys.exit(1)

    # ── sizes ─────────────────────────────────────────────────────────────────
    if args.mode == "sizes":
        if args.sizes_json:
            sizes = parse_sizes_from_gif_info_json(args.sizes_json)
        elif args.sizes:
            sizes = parse_sizes_arg(args.sizes)
        else:
            print("Error: --mode sizes requires --sizes-json or --sizes", file=sys.stderr)
            sys.exit(1)
        if not sizes:
            print("Error: no valid sizes found", file=sys.stderr)
            sys.exit(1)
        results, n_frames, src_size = mode_sizes(
            args.gif_path, sizes, args.chars, args.frame
        )
        print_results(results, "sizes", n_frames, src_size)

    # ── charsets ──────────────────────────────────────────────────────────────
    elif args.mode == "charsets":
        if args.cols is None or args.rows is None:
            print("Error: --mode charsets requires --cols and --rows", file=sys.stderr)
            sys.exit(1)
        if not args.charsets:
            print("Error: --mode charsets requires --charsets", file=sys.stderr)
            sys.exit(1)
        charsets = parse_charsets_arg(args.charsets)
        if not charsets:
            print("Error: no valid charsets found in --charsets", file=sys.stderr)
            sys.exit(1)
        results, n_frames, src_size = mode_charsets(
            args.gif_path, args.cols, args.rows, charsets, args.frame
        )
        print_results(results, "charsets", n_frames, src_size)

    # ── palettes ──────────────────────────────────────────────────────────────
    elif args.mode == "palettes":
        if args.cols is None or args.rows is None:
            print("Error: --mode palettes requires --cols and --rows", file=sys.stderr)
            sys.exit(1)
        if not args.palettes:
            print("Error: --mode palettes requires --palettes", file=sys.stderr)
            sys.exit(1)
        overrides = parse_palettes_arg(args.palettes)
        if not overrides:
            print("Error: no valid palette overrides found in --palettes", file=sys.stderr)
            sys.exit(1)
        results, n_frames, src_size, bg_hex, auto_pal = mode_palettes(
            args.gif_path, args.cols, args.rows, args.chars,
            overrides, args.frame, args.colors, args.bg_color
        )
        print_results(results, "palettes", n_frames, src_size)


if __name__ == "__main__":
    main()
