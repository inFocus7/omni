#!/usr/bin/env python3
"""
validate.py — Validate an OMNI ASCII animation folder before import.

Usage:
  validate.py <path-to-animation-or-pack-dir>

Checks:
  1. Folder structure — detects single (meta.json) vs pack (pack.json)
  2. JSON parsing — all config/frame files parse as valid JSON
  3. meta.json required fields — name, variants >= 1, each has size + frames_file
  4. pack.json — animations array non-empty, each listed subdir has meta.json
  5. Frame dimensions — each frame has exactly `rows` lines, each exactly `cols` visible chars
  6. Visible character counting — strips HTML tags, decodes entities
  7. Palette validation — class names + color values match allowed patterns
  8. frames_file existence — each variant's frames_file is present
  9. Fit ratio check (warning) — warns if cols/rows deviates >30% from ideal for the widget size

Exit codes:
  0 — all checks passed (warnings may be present)
  1 — at least one error
"""

import sys
import os
import json
import re
import math


# ── Regexes matching Go's sanitize.go ───────────────────────────────────────

CLASS_RE = re.compile(r'^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$')
COLOR_RE = re.compile(
    r'^(#[0-9a-fA-F]{3,8}'
    r'|rgb\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}\s*\)'
    r'|rgba\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*[0-9.]+\s*\)'
    r'|[a-zA-Z]{1,20})$'
)
TAG_RE = re.compile(r'<[^>]+>')
NUMERIC_ENTITY_RE = re.compile(r'&#\d+;')
HEX_ENTITY_RE = re.compile(r'&#x[0-9a-fA-F]+;')

GRID_COL_W = 161
GRID_ROW_H = 130
GRID_GAP   = 12
CHAR_W     = 0.6


# ── Helpers ──────────────────────────────────────────────────────────────────

def visible_len(html_str):
    """Count visible characters in an HTML row string."""
    text = TAG_RE.sub('', html_str)
    text = text.replace('&amp;', '&')
    text = text.replace('&lt;', '<')
    text = text.replace('&gt;', '>')
    text = text.replace('&nbsp;', ' ')
    text = NUMERIC_ENTITY_RE.sub('X', text)
    text = HEX_ENTITY_RE.sub('X', text)
    return len(text)


def pixel_dims(W, H):
    pxW = W * GRID_COL_W + (W - 1) * GRID_GAP
    pxH = H * GRID_ROW_H + (H - 1) * GRID_GAP
    return pxW, pxH


def ideal_ratio(size_str):
    """Return ideal cols/rows ratio k for a given size string like '2x1', or None."""
    m = re.match(r'^(\d+)x(\d+)$', size_str, re.IGNORECASE)
    if not m:
        return None
    W, H = int(m.group(1)), int(m.group(2))
    pxW, pxH = pixel_dims(W, H)
    if pxH == 0:
        return None
    return (pxW / pxH) / CHAR_W


# ── Validation logic ─────────────────────────────────────────────────────────

class Validator:
    def __init__(self, path):
        self.path = os.path.abspath(path)
        self.errors = []
        self.warnings = []
        self.checks_run = 0

    def error(self, msg):
        self.errors.append(f"  ERROR: {msg}")

    def warn(self, msg):
        self.warnings.append(f"  WARN:  {msg}")

    def ok(self, msg):
        print(f"  OK     {msg}")
        self.checks_run += 1

    def validate(self):
        if not os.path.isdir(self.path):
            self.error(f"not a directory: {self.path}")
            self._print_results()
            return len(self.errors) == 0

        # Detect single vs pack
        pack_json_path = os.path.join(self.path, "pack.json")
        meta_json_path = os.path.join(self.path, "meta.json")

        if os.path.isfile(pack_json_path):
            print(f"  Detected: PACK  ({self.path})\n")
            self._validate_pack(self.path, pack_json_path)
        elif os.path.isfile(meta_json_path):
            print(f"  Detected: SINGLE animation  ({self.path})\n")
            self._validate_single(self.path)
        else:
            self.error("no pack.json or meta.json found at top level")

        self._print_results()
        return len(self.errors) == 0

    def _validate_pack(self, pack_dir, pack_json_path):
        # Check 2: Parse pack.json
        data = self._read_json(pack_json_path, "pack.json")
        if data is None:
            return
        self.ok("pack.json parses as valid JSON")

        # Check: animations list non-empty
        animations = data.get("animations")
        if not isinstance(animations, list) or len(animations) == 0:
            self.error("pack.json: 'animations' must be a non-empty array")
            return
        self.ok(f"pack.json: animations array has {len(animations)} entries")

        # Check: each animation subdir has meta.json
        for anim_name in animations:
            anim_dir = os.path.join(pack_dir, anim_name)
            anim_meta = os.path.join(anim_dir, "meta.json")
            if not os.path.isdir(anim_dir):
                self.error(f"pack animation '{anim_name}': subdirectory not found")
                continue
            if not os.path.isfile(anim_meta):
                self.error(f"pack animation '{anim_name}': meta.json not found")
                continue
            self.ok(f"pack animation '{anim_name}': directory and meta.json present")
            self._validate_single(anim_dir, prefix=f"[{anim_name}] ")

    def _validate_single(self, anim_dir, prefix=""):
        meta_path = os.path.join(anim_dir, "meta.json")

        # Check: meta.json exists
        if not os.path.isfile(meta_path):
            self.error(f"{prefix}meta.json not found in {anim_dir}")
            return

        # Check: parse meta.json
        meta = self._read_json(meta_path, f"{prefix}meta.json")
        if meta is None:
            return
        self.ok(f"{prefix}meta.json parses as valid JSON")

        # Check: name
        name = meta.get("name", "")
        if not name:
            self.error(f"{prefix}meta.json: 'name' is missing or empty")
        else:
            self.ok(f"{prefix}meta.json: name = '{name}'")

        # Check: variants
        variants = meta.get("variants")
        if not isinstance(variants, list) or len(variants) == 0:
            self.error(f"{prefix}meta.json: 'variants' must be a non-empty array")
            return
        self.ok(f"{prefix}meta.json: {len(variants)} variant(s) defined")

        # Check: palette
        palette = meta.get("palette")
        if palette is not None:
            self._validate_palette(palette, prefix)

        # Check each variant
        for i, variant in enumerate(variants):
            self._validate_variant(anim_dir, variant, i, prefix)

    def _validate_palette(self, palette, prefix):
        if not isinstance(palette, dict):
            self.error(f"{prefix}palette must be an object")
            return
        bad = False
        for cls, color in palette.items():
            if not CLASS_RE.match(cls):
                self.error(
                    f"{prefix}palette: invalid class name '{cls}': "
                    "must match ^[a-zA-Z_][a-zA-Z0-9_-]{{0,63}}$"
                )
                bad = True
            if not COLOR_RE.match(str(color)):
                self.error(
                    f"{prefix}palette: invalid color '{color}' for class '{cls}': "
                    "must be hex (#RGB–#RRGGBBAA), rgb(), rgba(), or named CSS color"
                )
                bad = True
        if not bad:
            self.ok(f"{prefix}palette: {len(palette)} entry/entries valid")

    def _validate_variant(self, anim_dir, variant, idx, prefix):
        size = variant.get("size", "")
        frames_file = variant.get("frames_file", "")
        cols = variant.get("cols")
        rows = variant.get("rows")

        tag = f"{prefix}variant[{idx}]"

        # Required fields
        if not size:
            self.error(f"{tag}: 'size' is missing")
        if not frames_file:
            self.error(f"{tag}: 'frames_file' is missing")
            return
        if not isinstance(cols, int) or cols <= 0:
            self.error(f"{tag}: 'cols' must be a positive integer, got {cols!r}")
            return
        if not isinstance(rows, int) or rows <= 0:
            self.error(f"{tag}: 'rows' must be a positive integer, got {rows!r}")
            return

        self.ok(f"{tag}: size={size}, cols={cols}, rows={rows}, fps={variant.get('fps')}")

        # Check frames_file existence
        frames_path = os.path.join(anim_dir, frames_file)
        if not os.path.isfile(frames_path):
            self.error(f"{tag}: frames_file '{frames_file}' not found at {frames_path}")
            return
        self.ok(f"{tag}: frames_file '{frames_file}' exists")

        # Parse frames JSON
        frames = self._read_json(frames_path, f"{tag} frames file")
        if frames is None:
            return
        if not isinstance(frames, list) or len(frames) == 0:
            self.error(f"{tag}: frames file must be a non-empty JSON array")
            return
        self.ok(f"{tag}: frames file parses, {len(frames)} frame(s)")

        # Validate frame dimensions
        dim_errors = 0
        for fi, frame in enumerate(frames):
            if not isinstance(frame, str):
                self.error(f"{tag}: frame {fi} is not a string")
                dim_errors += 1
                continue
            lines = frame.split('\n')
            if len(lines) != rows:
                self.error(
                    f"{tag}: frame {fi} has {len(lines)} lines, expected {rows}"
                )
                dim_errors += 1
                continue
            for li, line in enumerate(lines):
                vlen = visible_len(line)
                if vlen != cols:
                    self.error(
                        f"{tag}: frame {fi} line {li} has {vlen} visible chars, "
                        f"expected {cols}"
                    )
                    dim_errors += 1
                    if dim_errors >= 10:
                        self.error(f"{tag}: too many dimension errors, stopping early")
                        return

        if dim_errors == 0:
            self.ok(f"{tag}: all frames have correct dimensions ({rows} lines × {cols} cols)")

        # Fit ratio check (warning only)
        if size:
            k = ideal_ratio(size)
            if k is not None and rows > 0:
                actual = cols / rows
                dev = abs(actual - k) / k if k > 0 else 0
                if dev > 0.30:
                    self.warn(
                        f"{tag}: cols/rows ratio {actual:.2f} deviates {dev*100:.0f}% from "
                        f"ideal {k:.2f} for size {size} — consider running fit.py"
                    )

    def _read_json(self, path, label):
        try:
            with open(path, 'r', encoding='utf-8') as f:
                return json.load(f)
        except json.JSONDecodeError as e:
            self.error(f"{label}: invalid JSON: {e}")
            return None
        except OSError as e:
            self.error(f"{label}: cannot read file: {e}")
            return None

    def _print_results(self):
        print()
        if self.warnings:
            for w in self.warnings:
                print(w)
            print()
        if self.errors:
            for e in self.errors:
                print(e)
            print()
            print(f"  FAILED  ({len(self.errors)} error(s), {len(self.warnings)} warning(s))")
        else:
            print(f"  PASSED  ({self.checks_run} checks, {len(self.warnings)} warning(s))")


# ── Entry point ───────────────────────────────────────────────────────────────

def main():
    if len(sys.argv) < 2:
        print("Usage: validate.py <path-to-animation-or-pack-dir>", file=sys.stderr)
        sys.exit(1)

    path = sys.argv[1]
    print(f"\nValidating: {os.path.abspath(path)}\n")
    v = Validator(path)
    ok = v.validate()
    print()
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
