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
 10. Size analysis — exact gzip size, spans/frame, run length, background ratio, recommendations

Exit codes:
  0 — all checks passed (warnings may be present)
  1 — at least one error
"""

import gzip
import json
import os
import re
import sys

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

# ── Shared recommendation thresholds ─────────────────────────────────────────
# These same thresholds are used by estimate.py (extrapolated) and validate.py
# (exact). They are also documented in both skill SKILL.md files so the agent
# knows them independently.
#
# Each entry: (metric_key, comparator, threshold, severity, message)
#   severity: 1 = ⚠ advisory, 2 = ⚠⚠ strong recommendation
#   comparator: 'gt', 'lt', 'gt_and_fps_gt'

THRESHOLDS = [
    # (label, check_fn, severity, recommendation)
    ('spans_high',    lambda m: m['avg_spans'] > 400,
     1, "High span count — reduce palette colors to increase run lengths"),
    ('spans_fps',     lambda m: m['avg_spans'] > 400 and m['fps'] > 12,
     2, "Heavy render load — reduce palette colors AND/OR target FPS"),
    ('run_short',     lambda m: 0 < m['avg_run'] < 3,
     1, "Short runs — color regions too fragmented; reduce palette or cols×rows"),
    ('run_severe',    lambda m: 0 < m['avg_run'] < 2,
     2, "Severe fragmentation — nearly per-character coloring; strongly reduce palette"),
    ('bg_low',        lambda m: m['bg_ratio'] < 0.20,
     1, "Low background ratio — verify background color is correct; more cells may be unspannable"),
    ('overhead_high', lambda m: m['overhead'] > 0.50,
     1, "Tag bytes dominate content — palette too large or background not unspanned"),
    ('gzip_large',    lambda m: m['gz_kb'] is not None and m['gz_kb'] > 500,
     2, "Large animation — reduce FPS, cols×rows, or color count"),
    ('gzip_moderate', lambda m: m['gz_kb'] is not None and 200 < m['gz_kb'] <= 500,
     1, "Moderate size — worth reviewing if smaller settings are acceptable"),
    ('frames_many',   lambda m: m['frame_count'] > 40,
     1, "Many frames — consider reducing target FPS to drop frames (~linear savings)"),
    ('palette_large', lambda m: m['palette_colors'] > 8,
     1, "Palette above recommended range (3–8) — expect shorter runs; monitor span count"),
]


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


def count_unspanned_spaces(frame_str):
    """
    Count spaces that appear outside any <span>...</span> tag.
    Iterates the string tracking depth; spaces at depth 0 are unspanned.
    """
    count = 0
    in_tag = False
    span_depth = 0
    i = 0
    s = frame_str
    n = len(s)
    while i < n:
        if s[i] == '<':
            in_tag = True
            # Peek ahead to determine open vs close span
            rest = s[i:]
            if rest.startswith('</span') or rest.startswith('</SPAN'):
                span_depth = max(0, span_depth - 1)
            elif rest.lower().startswith('<span'):
                span_depth += 1
            i += 1
            continue
        if in_tag:
            if s[i] == '>':
                in_tag = False
            i += 1
            continue
        if s[i] == ' ' and span_depth == 0:
            count += 1
        i += 1
    return count


def compute_size_metrics(frames, fps, palette_colors, frames_path=None):
    """
    Compute size and efficiency metrics for a list of frame strings.

    frames:         list of frame HTML strings (already parsed)
    fps:            frames per second (for render-load threshold)
    palette_colors: number of palette colors defined in meta.json
    frames_path:    if provided, measure exact file size + actual gzip bytes
                    if None (estimate mode), extrapolate from sample frames
    """
    spans_per_frame = []
    total_visible_chars = 0
    total_unspanned_spaces = 0
    total_raw_bytes = 0

    for frame in frames:
        span_count = frame.count('<span')
        spans_per_frame.append(span_count)
        visible = TAG_RE.sub('', frame).replace('\n', '')
        total_visible_chars += len(visible)
        total_unspanned_spaces += count_unspanned_spaces(frame)
        total_raw_bytes += len(frame.encode('utf-8'))

    frame_count = len(frames)
    avg_spans = sum(spans_per_frame) / frame_count if frame_count else 0
    max_spans = max(spans_per_frame) if spans_per_frame else 0
    min_spans = min(spans_per_frame) if spans_per_frame else 0

    # Average run length: visible chars per span across all frames
    total_spans = sum(spans_per_frame)
    avg_run = (total_visible_chars / total_spans) if total_spans > 0 else float('inf')

    bg_ratio = total_unspanned_spaces / total_visible_chars if total_visible_chars else 0
    overhead = 1.0 - (total_visible_chars / total_raw_bytes) if total_raw_bytes else 0

    if frames_path and os.path.isfile(frames_path):
        raw_kb = os.path.getsize(frames_path) / 1024
        with open(frames_path, 'rb') as f:
            gz_kb = len(gzip.compress(f.read())) / 1024
        gz_range = None  # exact, no range needed
    else:
        # Estimate mode: extrapolate from sample
        sample_raw = total_raw_bytes / frame_count if frame_count else 0
        raw_kb = (sample_raw * frame_count) / 1024
        gz_kb = None
        gz_range = (raw_kb * 0.20, raw_kb * 0.30)

    return {
        'frame_count':    frame_count,
        'fps':            fps,
        'palette_colors': palette_colors,
        'avg_spans':      avg_spans,
        'max_spans':      max_spans,
        'min_spans':      min_spans,
        'avg_run':        avg_run,
        'bg_ratio':       bg_ratio,
        'overhead':       overhead,
        'raw_kb':         raw_kb,
        'gz_kb':          gz_kb,
        'gz_range':       gz_range,
    }


def apply_thresholds(metrics):
    """
    Returns list of (severity, message) for each triggered threshold.
    Skips lower-severity entry when a higher-severity entry for the same
    root condition is also triggered (e.g. run_severe supersedes run_short).
    """
    triggered = []
    # Higher severity first so we can suppress lower-severity duplicates
    sorted_thresh = sorted(THRESHOLDS, key=lambda t: -t[2])
    suppress = set()
    for label, check_fn, severity, msg in sorted_thresh:
        try:
            fired = check_fn(metrics)
        except Exception:
            fired = False
        if fired:
            # Suppress lower-severity sibling if higher already fired
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
    # Deduplicate while preserving order
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
    lines.append(f"\n  Size analysis  ({frames_file} — {metrics['frame_count']} frames, {cols}×{rows})")
    lines.append("  " + "─" * 54)

    lines.append(f"  Raw JSON:          {metrics['raw_kb']:.1f} KB")
    if metrics['gz_kb'] is not None:
        lines.append(f"  Gzip (actual):     {metrics['gz_kb']:.1f} KB")
    else:
        lo, hi = metrics['gz_range']
        lines.append(f"  Gzip (est):        ~{lo:.1f}–{hi:.1f} KB  (20–30% heuristic)")

    lines.append(
        f"  Spans/frame:       avg {metrics['avg_spans']:.0f}"
        f"  max {metrics['max_spans']}"
        f"  min {metrics['min_spans']}"
    )
    if metrics['avg_run'] == float('inf'):
        lines.append("  Avg run length:    n/a (no spans)")
    else:
        lines.append(f"  Avg run length:    {metrics['avg_run']:.1f} chars")
    lines.append(f"  Background ratio:  {metrics['bg_ratio']*100:.0f}%  (unspanned spaces / total chars)")
    lines.append(f"  Overhead ratio:    {metrics['overhead']*100:.0f}%  (tag bytes / total frame bytes)")

    lines.append("")
    recs = apply_thresholds(metrics)
    if not recs:
        lines.append("  ✓ All size metrics look healthy")
    else:
        # Print healthy metrics first (those with no threshold fired)
        healthy = []
        if not any('span' in m.lower() for _, m in recs):
            healthy.append(f"✓ Span count is healthy (avg {metrics['avg_spans']:.0f}/frame)")
        if not any('run' in m.lower() for _, m in recs):
            healthy.append(f"✓ Run length is healthy ({metrics['avg_run']:.1f} chars)")
        if not any('background' in m.lower() for _, m in recs):
            healthy.append(f"✓ Background ratio is healthy ({metrics['bg_ratio']*100:.0f}%)")
        if metrics['gz_kb'] is not None and not any('large' in m.lower() or 'moderate' in m.lower() for _, m in recs):
            healthy.append(f"✓ Gzip size is healthy ({metrics['gz_kb']:.1f} KB)")
        for h in healthy:
            lines.append(f"  {h}")
        for sev, msg in recs:
            sym = "⚠⚠" if sev == 2 else "⚠ "
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
        palette_colors = 0
        if palette is not None:
            self._validate_palette(palette, prefix)
            palette_colors = len(palette) if isinstance(palette, dict) else 0

        for i, variant in enumerate(variants):
            self._validate_variant(anim_dir, variant, i, prefix, palette_colors)

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

    def _validate_variant(self, anim_dir, variant, idx, prefix, palette_colors=0):
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

        frames = self._read_json(frames_path, f"{tag} frames file")
        if frames is None:
            print(f"\n  (size analysis skipped — frame parsing failed for {frames_file})")
            return
        if not isinstance(frames, list) or len(frames) == 0:
            self.error(f"{tag}: frames file must be a non-empty JSON array")
            print(f"\n  (size analysis skipped — empty frames array in {frames_file})")
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
                        print(f"\n  (size analysis skipped — dimension errors in {frames_file})")
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

        # Size analysis (always shown, even when all healthy)
        if dim_errors == 0:
            metrics = compute_size_metrics(
                frames, fps=fps, palette_colors=palette_colors,
                frames_path=frames_path
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
