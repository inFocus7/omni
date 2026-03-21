#!/usr/bin/env python3
"""
validate.py — Validate an ICG ASCII animation folder before import.

Usage:
  validate.py <path-to-animation-or-pack-dir>

Checks:
  1. Folder structure — detects single (meta.json) vs pack (pack.json)
  2. JSON parsing — all config/frame files parse as valid JSON
  3. meta.json required fields — name, variants >= 1, each has size + frames_file
  4. pack.json — animations array non-empty, each listed subdir has meta.json
  5. ICG structure — frames file has class_table + frames array
  6. class_table validation — entries match CSS class name regex
  7. Frame chars dimensions — each frame has exactly `rows` newline-separated lines, each `cols` chars
  8. Frame colors — base64 decodes to correct length (cols * rows), values within class_table range
  9. Fit ratio check (warning) — warns if cols/rows deviates >30% from ideal for widget size
 10. Size analysis — chars size, colors size, gzip estimate, recommendations

Exit codes:
  0 — all checks passed (warnings may be present)
  1 — at least one error
"""

import base64
import gzip
import json
import os
import re
import sys

# ── Regexes ──────────────────────────────────────────────────────────────────

# class_table[0] is always "" (default/no color). Non-empty entries must be
# valid CSS class names.
CLASS_RE = re.compile(r'^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$')

GRID_COL_W = 161
GRID_ROW_H = 130
GRID_GAP   = 12
CHAR_W     = 0.6

# ── Shared recommendation thresholds ─────────────────────────────────────────

THRESHOLDS = [
    # (label, check_fn, severity, recommendation)
    ('gzip_large',    lambda m: m['gz_kb'] is not None and m['gz_kb'] > 500,
     2, "Large animation — reduce FPS, cols*rows, or color count"),
    ('gzip_moderate', lambda m: m['gz_kb'] is not None and 200 < m['gz_kb'] <= 500,
     1, "Moderate size — worth reviewing if smaller settings are acceptable"),
    ('frames_many',   lambda m: m['frame_count'] > 40,
     1, "Many frames — consider reducing target FPS to drop frames (~linear savings)"),
    ('palette_large', lambda m: m['class_count'] > 8,
     1, "class_table above recommended range (3-8) — monitor file size"),
    ('colors_large',  lambda m: m['avg_colors_kb'] > 5,
     1, "Large color data per frame — reduce cols*rows or class_table entries"),
]


# ── Helpers ──────────────────────────────────────────────────────────────────

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


def compute_icg_metrics(icg_data, fps, frames_path=None):
    """
    Compute size and efficiency metrics for an ICG frames file.

    icg_data:    parsed ICG dict with class_table and frames
    fps:         frames per second
    frames_path: if provided, measure exact file size + actual gzip bytes
    """
    class_table = icg_data.get('class_table', [])
    frames = icg_data.get('frames', [])
    frame_count = len(frames)

    total_chars_bytes = 0
    total_colors_bytes = 0
    default_cells = 0
    total_cells = 0

    for frame in frames:
        chars = frame.get('chars', '')
        colors_b64 = frame.get('colors', '')
        total_chars_bytes += len(chars.encode('utf-8'))
        try:
            color_bytes = base64.b64decode(colors_b64)
        except Exception:
            color_bytes = b''
        total_colors_bytes += len(color_bytes)
        total_cells += len(color_bytes)
        default_cells += sum(1 for b in color_bytes if b == 0)

    avg_chars_kb = (total_chars_bytes / frame_count / 1024) if frame_count else 0
    avg_colors_kb = (total_colors_bytes / frame_count / 1024) if frame_count else 0
    default_ratio = default_cells / total_cells if total_cells else 0

    if frames_path and os.path.isfile(frames_path):
        raw_kb = os.path.getsize(frames_path) / 1024
        with open(frames_path, 'rb') as f:
            gz_kb = len(gzip.compress(f.read())) / 1024
        gz_range = None
    else:
        raw_bytes = total_chars_bytes + total_colors_bytes
        # Add ~30% for JSON overhead (keys, base64 encoding, etc.)
        raw_kb = (raw_bytes * 1.3) / 1024
        gz_kb = None
        gz_range = (raw_kb * 0.20, raw_kb * 0.40)

    return {
        'frame_count':    frame_count,
        'fps':            fps,
        'class_count':    len(class_table),
        'avg_chars_kb':   avg_chars_kb,
        'avg_colors_kb':  avg_colors_kb,
        'default_ratio':  default_ratio,
        'raw_kb':         raw_kb,
        'gz_kb':          gz_kb,
        'gz_range':       gz_range,
    }


def apply_thresholds(metrics):
    """
    Returns list of (severity, message) for each triggered threshold.
    Skips lower-severity entry when a higher-severity entry for the same
    root condition is also triggered.
    """
    triggered = []
    sorted_thresh = sorted(THRESHOLDS, key=lambda t: -t[2])
    suppress = set()
    for label, check_fn, severity, msg in sorted_thresh:
        try:
            fired = check_fn(metrics)
        except Exception:
            fired = False
        if fired:
            root = label.rsplit('_', 1)[0]
            if root in suppress:
                continue
            if severity == 2:
                suppress.add(root)
            triggered.append((severity, msg))
    # Return in threshold definition order for stable output
    ordered = []
    for label, check_fn, severity, msg in THRESHOLDS:
        if any(m == msg for _, m in triggered):
            ordered.append((severity, msg))
    seen = set()
    result = []
    for sev, msg in ordered:
        if msg not in seen:
            seen.add(msg)
            result.append((sev, msg))
    return result


def format_size_analysis(metrics, variant_tag, frames_file, cols, rows):
    """Format the size analysis block as a printable string."""
    lines = []
    lines.append(f"\n  Size analysis  ({frames_file} -- {metrics['frame_count']} frames, {cols}x{rows})")
    lines.append("  " + "-" * 54)

    lines.append(f"  Raw JSON:          {metrics['raw_kb']:.1f} KB")
    if metrics['gz_kb'] is not None:
        lines.append(f"  Gzip (actual):     {metrics['gz_kb']:.1f} KB")
    else:
        lo, hi = metrics['gz_range']
        lines.append(f"  Gzip (est):        ~{lo:.1f}-{hi:.1f} KB  (20-40% heuristic)")

    lines.append(f"  class_table:       {metrics['class_count']} entries")
    lines.append(f"  Avg chars/frame:   {metrics['avg_chars_kb']:.2f} KB")
    lines.append(f"  Avg colors/frame:  {metrics['avg_colors_kb']:.2f} KB")
    lines.append(f"  Default ratio:     {metrics['default_ratio']*100:.0f}%  (cells with class_table[0])")

    lines.append("")
    recs = apply_thresholds(metrics)
    if not recs:
        lines.append("  OK All size metrics look healthy")
    else:
        healthy = []
        if metrics['gz_kb'] is not None and not any('large' in m.lower() or 'moderate' in m.lower() for _, m in recs):
            healthy.append(f"OK Gzip size is healthy ({metrics['gz_kb']:.1f} KB)")
        if not any('class_table' in m.lower() or 'palette' in m.lower() for _, m in recs):
            healthy.append(f"OK class_table size is healthy ({metrics['class_count']} entries)")
        for h in healthy:
            lines.append(f"  {h}")
        for sev, msg in recs:
            sym = "!!" if sev == 2 else "! "
            lines.append(f"  {sym} {msg}")

    return '\n'.join(lines)


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
        data = self._read_json(pack_json_path, "pack.json")
        if data is None:
            return
        self.ok("pack.json parses as valid JSON")

        animations = data.get("animations")
        if not isinstance(animations, list) or len(animations) == 0:
            self.error("pack.json: 'animations' must be a non-empty array")
            return
        self.ok(f"pack.json: animations array has {len(animations)} entries")

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

        if not os.path.isfile(meta_path):
            self.error(f"{prefix}meta.json not found in {anim_dir}")
            return

        meta = self._read_json(meta_path, f"{prefix}meta.json")
        if meta is None:
            return
        self.ok(f"{prefix}meta.json parses as valid JSON")

        name = meta.get("name", "")
        if not name:
            self.error(f"{prefix}meta.json: 'name' is missing or empty")
        else:
            self.ok(f"{prefix}meta.json: name = '{name}'")

        variants = meta.get("variants")
        if not isinstance(variants, list) or len(variants) == 0:
            self.error(f"{prefix}meta.json: 'variants' must be a non-empty array")
            return
        self.ok(f"{prefix}meta.json: {len(variants)} variant(s) defined")

        palette = meta.get("palette")
        if palette is not None:
            self._validate_palette(palette, prefix)

        for i, variant in enumerate(variants):
            self._validate_variant(anim_dir, variant, i, prefix)

    def _validate_palette(self, palette, prefix):
        """Validate the palette in meta.json (maps class names to CSS colors)."""
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
        if not bad:
            self.ok(f"{prefix}palette: {len(palette)} entry/entries valid")

    def _validate_variant(self, anim_dir, variant, idx, prefix):
        size = variant.get("size", "")
        frames_file = variant.get("frames_file", "")
        cols = variant.get("cols")
        rows = variant.get("rows")
        fps  = variant.get("fps", 0) or 0

        tag = f"{prefix}variant[{idx}]"

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

        self.ok(f"{tag}: size={size}, cols={cols}, rows={rows}, fps={fps}")

        frames_path = os.path.join(anim_dir, frames_file)
        if not os.path.isfile(frames_path):
            self.error(f"{tag}: frames_file '{frames_file}' not found at {frames_path}")
            return
        self.ok(f"{tag}: frames_file '{frames_file}' exists")

        icg_data = self._read_json(frames_path, f"{tag} frames file")
        if icg_data is None:
            print(f"\n  (size analysis skipped -- frame parsing failed for {frames_file})")
            return

        # ── Validate ICG structure ───────────────────────────────────────
        if not isinstance(icg_data, dict):
            self.error(f"{tag}: frames file must be a JSON object (ICG format)")
            return

        class_table = icg_data.get("class_table")
        frames = icg_data.get("frames")

        if not isinstance(class_table, list):
            self.error(f"{tag}: 'class_table' must be an array")
            return
        if len(class_table) == 0:
            self.error(f"{tag}: 'class_table' must be non-empty (at least [\"\"] for default)")
            return
        if not isinstance(frames, list) or len(frames) == 0:
            self.error(f"{tag}: 'frames' must be a non-empty array")
            return

        self.ok(f"{tag}: ICG structure valid (class_table: {len(class_table)}, frames: {len(frames)})")

        # ── Validate class_table entries ─────────────────────────────────
        ct_errors = 0
        for ci, entry in enumerate(class_table):
            if not isinstance(entry, str):
                self.error(f"{tag}: class_table[{ci}] must be a string, got {type(entry).__name__}")
                ct_errors += 1
                continue
            if ci == 0:
                # class_table[0] must be "" (default / no color)
                if entry != "":
                    self.error(f"{tag}: class_table[0] must be \"\" (default color), got {entry!r}")
                    ct_errors += 1
            else:
                if not CLASS_RE.match(entry):
                    self.error(
                        f"{tag}: class_table[{ci}] = {entry!r} is not a valid CSS class name: "
                        "must match ^[a-zA-Z_][a-zA-Z0-9_-]{{0,63}}$"
                    )
                    ct_errors += 1

        if ct_errors == 0:
            self.ok(f"{tag}: class_table entries valid ({len(class_table)} classes)")

        # ── Check that class_table entries referenced by palette exist ────
        # (palette is in meta.json, class_table is in ICG file)
        meta_path = os.path.join(anim_dir, "meta.json")
        meta = self._read_json(meta_path, f"{tag} meta.json (palette cross-ref)")
        if meta is not None:
            palette = meta.get("palette")
            if isinstance(palette, dict):
                ct_set = set(class_table)
                missing = [cls for cls in palette if cls not in ct_set]
                if missing:
                    for cls in missing:
                        self.error(
                            f"{tag}: palette class '{cls}' not found in class_table"
                        )
                else:
                    self.ok(f"{tag}: all palette classes present in class_table")

        # ── Validate each frame ──────────────────────────────────────────
        expected_cells = cols * rows
        dim_errors = 0
        color_errors = 0

        for fi, frame in enumerate(frames):
            if not isinstance(frame, dict):
                self.error(f"{tag}: frame {fi} must be an object with 'chars' and 'colors'")
                dim_errors += 1
                continue

            chars = frame.get("chars")
            colors_b64 = frame.get("colors")

            if not isinstance(chars, str):
                self.error(f"{tag}: frame {fi} 'chars' must be a string")
                dim_errors += 1
                continue
            if not isinstance(colors_b64, str):
                self.error(f"{tag}: frame {fi} 'colors' must be a base64 string")
                color_errors += 1
                continue

            # Validate chars dimensions
            char_lines = chars.split('\n')
            if len(char_lines) != rows:
                self.error(
                    f"{tag}: frame {fi} has {len(char_lines)} lines, expected {rows}"
                )
                dim_errors += 1
                if dim_errors >= 10:
                    self.error(f"{tag}: too many dimension errors, stopping early")
                    return
                continue

            line_ok = True
            for li, line in enumerate(char_lines):
                if len(line) != cols:
                    self.error(
                        f"{tag}: frame {fi} line {li} has {len(line)} chars, expected {cols}"
                    )
                    dim_errors += 1
                    line_ok = False
                    if dim_errors >= 10:
                        self.error(f"{tag}: too many dimension errors, stopping early")
                        return

            # Validate colors base64
            try:
                color_bytes = base64.b64decode(colors_b64)
            except Exception as e:
                self.error(f"{tag}: frame {fi} 'colors' is not valid base64: {e}")
                color_errors += 1
                if color_errors >= 10:
                    self.error(f"{tag}: too many color errors, stopping early")
                    return
                continue

            if len(color_bytes) != expected_cells:
                self.error(
                    f"{tag}: frame {fi} colors decoded to {len(color_bytes)} bytes, "
                    f"expected {expected_cells} (cols={cols} * rows={rows})"
                )
                color_errors += 1
                if color_errors >= 10:
                    self.error(f"{tag}: too many color errors, stopping early")
                    return
                continue

            # Validate color byte values are within class_table range
            max_idx = len(class_table) - 1
            for bi, b in enumerate(color_bytes):
                if b > max_idx:
                    self.error(
                        f"{tag}: frame {fi} color byte [{bi}] = {b} exceeds "
                        f"class_table max index {max_idx}"
                    )
                    color_errors += 1
                    if color_errors >= 10:
                        self.error(f"{tag}: too many color errors, stopping early")
                        return
                    break  # one error per frame is enough for out-of-range

        if dim_errors == 0:
            self.ok(f"{tag}: all frames have correct char dimensions ({rows} lines x {cols} cols)")
        if color_errors == 0:
            self.ok(f"{tag}: all frames have valid colors (base64, correct length, in-range)")

        # Fit ratio check (warning only)
        if size:
            k = ideal_ratio(size)
            if k is not None and rows > 0:
                actual = cols / rows
                dev = abs(actual - k) / k if k > 0 else 0
                if dev > 0.30:
                    self.warn(
                        f"{tag}: cols/rows ratio {actual:.2f} deviates {dev*100:.0f}% from "
                        f"ideal {k:.2f} for size {size} -- consider running fit.py"
                    )

        # Size analysis
        if dim_errors == 0 and color_errors == 0:
            metrics = compute_icg_metrics(
                icg_data, fps=fps, frames_path=frames_path
            )
            print(format_size_analysis(metrics, tag, frames_file, cols, rows))

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
