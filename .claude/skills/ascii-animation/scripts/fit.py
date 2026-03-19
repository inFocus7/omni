#!/usr/bin/env python3
"""
fit.py — Compute recommended cols/rows for an OMNI ASCII animation widget.

Usage:
  fit.py <WxH>                    # suggestions using default C=80, R=24
  fit.py <WxH> --cols <C>         # anchor cols, compute rows
  fit.py <WxH> --rows <R>         # anchor rows, compute cols

Examples:
  fit.py 2x1
  fit.py 2x2 --cols 60
  fit.py 1x2 --rows 30
"""

import sys
import math
import argparse


GRID_COL_W = 161
GRID_ROW_H = 130
GRID_GAP   = 12
CHAR_W     = 0.6


def pixel_dims(W, H):
    pxW = W * GRID_COL_W + (W - 1) * GRID_GAP
    pxH = H * GRID_ROW_H + (H - 1) * GRID_GAP
    return pxW, pxH


def font_size(cols, rows, pxW, pxH):
    if cols <= 0 or rows <= 0:
        return 0
    return math.floor(min(pxW / (cols * CHAR_W), pxH / rows))


def compute_suggestions(W, H, C=80, R=24):
    pxW, pxH = pixel_dims(W, H)
    k = (pxW / pxH) / CHAR_W  # ideal cols/rows ratio

    rowsB = max(1, round((k * C + R) / (k * k + 1)))
    suggestions = [
        {"label": "Keep rows", "cols": max(1, round(R * k)), "rows": R},
        {"label": "Keep cols", "cols": C,                    "rows": max(1, round(C / k))},
        {"label": "Balanced",  "cols": max(1, round(k * rowsB)), "rows": rowsB},
    ]

    # Deduplicate by cols×rows
    seen = set()
    unique = []
    for s in suggestions:
        key = (s["cols"], s["rows"])
        if key not in seen:
            seen.add(key)
            unique.append(s)
    return unique, k, pxW, pxH


def main():
    parser = argparse.ArgumentParser(
        description="Compute recommended cols/rows for an OMNI ASCII animation widget."
    )
    parser.add_argument("size", help="Widget size in WxH format, e.g. 2x1")
    parser.add_argument("--cols", type=int, default=None, help="Anchor cols value")
    parser.add_argument("--rows", type=int, default=None, help="Anchor rows value")
    args = parser.parse_args()

    # Parse size
    parts = args.size.lower().split("x")
    if len(parts) != 2:
        print(f"Error: size must be WxH (e.g. 2x1), got: {args.size}", file=sys.stderr)
        sys.exit(1)
    try:
        W, H = int(parts[0]), int(parts[1])
        if W < 1 or H < 1:
            raise ValueError
    except ValueError:
        print(f"Error: W and H must be positive integers, got: {args.size}", file=sys.stderr)
        sys.exit(1)

    C = args.cols if args.cols is not None else 80
    R = args.rows if args.rows is not None else 24

    pxW, pxH = pixel_dims(W, H)
    suggestions, k, _, _ = compute_suggestions(W, H, C, R)

    print(f"\n  Widget size : {W}x{H}")
    print(f"  Pixel dims  : {pxW} × {pxH} px")
    print(f"  Ideal ratio : {k:.3f}  (cols/rows for perfect fill with CHAR_W=0.6)")
    print(f"  Anchors     : cols={C}, rows={R}")
    print()
    print(f"  {'Label':<12}  {'cols':>6}  {'rows':>6}  {'font (px)':>10}  {'cols×rows':>12}")
    print(f"  {'-'*12}  {'-'*6}  {'-'*6}  {'-'*10}  {'-'*12}")
    for s in suggestions:
        fs = font_size(s["cols"], s["rows"], pxW, pxH)
        grid = f"{s['cols']}×{s['rows']}"
        print(f"  {s['label']:<12}  {s['cols']:>6}  {s['rows']:>6}  {fs:>9}px  {grid:>12}")
    print()
    print("  Tip: more cols/rows = smaller font (more detail)")
    print("       fewer cols/rows = larger font (bolder look)")

    if args.cols is not None and args.rows is not None:
        # Custom: both anchors specified, show what that gives
        fs = font_size(args.cols, args.rows, pxW, pxH)
        ratio = args.cols / args.rows if args.rows > 0 else 0
        dev = abs(ratio - k) / k if k > 0 else 0
        print(f"\n  Custom {args.cols}×{args.rows}: font={fs}px, ratio={ratio:.3f} "
              f"({'%.0f' % (dev*100)}% deviation from ideal)")
        if dev > 0.30:
            print("  Warning: this ratio deviates >30% from ideal — widget may not fill well.")
    print()


if __name__ == "__main__":
    main()
