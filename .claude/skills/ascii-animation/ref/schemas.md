# ASCII Animation Schemas

Complete field-by-field reference for all JSON files used by OMNI's ASCII animation import system.

---

## meta.json (PackMeta)

Describes a single animation and its size variants.

```json
{
  "name": "my-animation",
  "palette": {
    "className": "colorValue"
  },
  "variants": [
    {
      "size": "1x1",
      "cols": 45,
      "rows": 22,
      "fps": 8,
      "frames_file": "frames-1x1.json"
    }
  ]
}
```

### Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | **yes** | Non-empty. Used as the animation identifier. |
| `palette` | object | no | Map of CSS class name → CSS color value. Omit for monochrome. |
| `variants` | array | **yes** | At least one variant required. |

### VariantFileMeta (each entry in `variants`)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `size` | string | **yes** | Widget grid size, e.g. `"1x1"`, `"2x2"`. Format: `WxH`. |
| `cols` | int | **yes** | Number of character columns per frame line. |
| `rows` | int | **yes** | Number of lines per frame. |
| `fps` | int | **yes** | Playback speed in frames per second. |
| `frames_file` | string | **yes** | Filename (relative to animation dir) for the frames JSON. Convention: `frames-<size>.json`. |

### Validation errors from `ParseMetaJSON`
- `"meta.json: missing name field"` — `name` is empty or absent
- `"meta.json: no variants defined"` — `variants` is empty or absent
- `"meta.json: variant N missing size"` — variant at index N has no `size`
- `"meta.json: variant \"1x1\" missing frames_file"` — variant has no `frames_file`

---

## frames-<size>.json

Holds the actual animation frame data for one size variant.

```json
[
  "frame0_html_string",
  "frame1_html_string",
  "frame2_html_string"
]
```

### Structure
- JSON array of strings
- Each string is one frame's HTML content
- Must contain at least 1 frame (empty array is an error)
- Each frame string: `rows` lines joined by `\n`
- Each line: exactly `cols` **visible characters** (text only, no HTML tags)

### Frame HTML rules

**Allowed:**
- `<span class="paletteName">...</span>` — colors the enclosed text
- `<br>` — inserts a visual line break (prefer `\n` for frame line separation)
- Plain text characters

**Stripped by sanitizer (not an error, just removed):**
- Any element not listed as allowed (e.g. `<div>`, `<p>`, `<b>`) — its text content is kept, tags removed

**Stripped entirely (content also removed):**
- `<script>`, `<style>`, `<svg>`, `<math>`, `<template>`, `<iframe>`, `<object>`, `<embed>`

**Palette class validation (SanitizePalette):**
- Class name regex: `^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$`
  - Must start with letter or underscore
  - May contain letters, digits, hyphens, underscores
  - Max 64 characters total
- Color value regex: `^(#[0-9a-fA-F]{3,8}|rgb\(...\)|rgba\(...\)|[a-zA-Z]{1,20})$`
  - Hex: `#RGB`, `#RRGGBB`, `#RRGGBBAA` (3–8 hex digits after `#`)
  - RGB: `rgb(r, g, b)` with optional spaces around values
  - RGBA: `rgba(r, g, b, a)` with float alpha
  - Named: CSS color name, letters only, 1–20 characters (e.g. `red`, `forestgreen`)

### Go struct (AnimationVariant in store.go)
```go
type AnimationVariant struct {
    Name       string            // animation identifier
    Source     string            // "" for local, URL for remote
    Size       string            // "WxH"
    Cols       int
    Rows       int
    FPS        int
    Palette    map[string]string
    FirstFrame string            // first frame as plain string (set on import)
    FramesGzip []byte            // gzip-compressed JSON array of frames
}
```
`FramesGzip` and `FirstFrame` are computed by the import handler; you never write them in files.

---

## pack.json (PackJSON)

Describes a multi-animation bundle. Only present in pack imports.

```json
{
  "name": "my-pack",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "A collection of animations",
  "license": "MIT",
  "animations": ["animation-one", "animation-two"]
}
```

### Fields

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `animations` | array of strings | **yes** | Non-empty. Each entry is a subdirectory name containing `meta.json`. |
| `name` | string | no | Human-readable pack name. |
| `version` | string | no | Semver string, e.g. `"1.0.0"`. |
| `author` | string | no | Creator name. |
| `description` | string | no | Short description. |
| `license` | string | no | License identifier, e.g. `"MIT"`. |

### Validation errors from `ParsePackJSON`
- `"pack.json: animations list must not be empty"` — `animations` is empty or absent

---

## Import detection logic

The import handler (`pkg/api/ascii.go`) auto-detects format:

- **Pack**: if any uploaded file has basename `pack.json` at depth 2 (e.g. `my-pack/pack.json`)
- **Single**: otherwise (looks for `meta.json` anywhere in the upload)

### Single animation folder structure
```
my-animation/
├── meta.json
└── frames-1x1.json        ← referenced by meta.json variant.frames_file
```

### Pack folder structure
```
my-pack/
├── pack.json              ← lists ["anim-a", "anim-b"]
├── anim-a/
│   ├── meta.json
│   └── frames-1x1.json
└── anim-b/
    ├── meta.json
    └── frames-2x1.json
```

The `animations` array in `pack.json` must exactly match the subdirectory names.

---

## Export format

When exporting via OMNI's UI (`/ascii` → Export), the server produces a zip with:
- Single animation (1 selected): `<name>/meta.json` + `<name>/frames-<size>.json`
- Multiple animations (2+): `<pack-name>/pack.json` + `<pack-name>/<anim>/meta.json` + frames files

The frames filename is always derived as `frames-<size>.json` (e.g. `frames-1x1.json` for size `1x1`).
