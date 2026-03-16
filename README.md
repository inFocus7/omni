<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/omni-white.png" />
    <source media="(prefers-color-scheme: light)" srcset="assets/omni-black.png" />
    <img alt="Omni" src="assets/omni-black.png" height="256" />
  </picture>
</p>

<p align="center">A personal developer dashboard for tracking contributions and team activity.</p>

<p align="center">
  <img alt="GIF Demo of OMNI" src="assets/omni-record.gif" width="800" />
</p>

## Running

Requires Go 1.26+.

```sh
export GITHUB_TOKEN="your_token_here"
go run ./app
```

The server starts on `:8080` by default.

### GitHub Token

You'll need a [personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) set as `GITHUB_TOKEN`.

Recommended setup:
- Classic token
- 30-day expiration
- Scopes: **repo**, **read:user**

### Docker

```sh
make docker-build
docker run -e GITHUB_TOKEN="your_token_here" -p 8080:8080 omni
```

### Development (live reload)

```sh
make check-deps   # installs air + trivy
make run-live
```

## Plugins

Omni is built around a widget registry. Each plugin provides one or more widgets that users can pin, resize, and arrange on their dashboard.

### GitHub

Tracks your GitHub activity. Widgets include:

| Widget | Description |
|---|---|
| `github-authored` | Count of PRs you authored |
| `github-reviewed` | Count of PRs you reviewed |
| `github-ratio` | Author-to-reviewer ratio with approval rate |
| `github-rightnow` | Live summary — open PRs, review requests, assigned issues |

All GitHub widgets support time filters: `1d`, `7d`, `1mo`, `ytd`, `all`.

**Watching orgs and repos**

Go to Settings → GitHub to add entries in GitHub Search qualifier format:

```
org:myorg           # all repos in an org
repo:owner/repo     # a specific repo
```

### ASCII

Renders animated ASCII art as dashboard widgets. Built-in animations are embedded at startup. You can also load external animations at runtime by pointing `OMNI_DATA_DIR` to a directory:

```sh
export OMNI_DATA_DIR="/path/to/my/data"
# Omni will load animations from $OMNI_DATA_DIR/ascii/
```

External animations override built-ins by name.

Each animation lives in its own subdirectory and requires a `meta.json`:

```json
{
  "name": "my-anim",
  "size": "2x1",
  "cols": 40,
  "rows": 10,
  "fps": 12,
  "palette": ["#00ff00", "#ffffff"],
  "frames": ["<span class=\"ac0\">frame 0 html</span>", "..."]
}
```

| Field | Description |
|---|---|
| `name` | Unique animation name (used as widget ID `ascii-{name}`) |
| `size` | Grid dimensions as `WxH`, e.g. `2x1` |
| `cols` / `rows` | Character dimensions of the animation |
| `fps` | Playback speed |
| `palette` | Optional color palette — classes `.ac0`, `.ac1`, … map to these colors |
| `frames` | Pre-rendered HTML strings, one per frame |

Animation frames are served via `GET /api/ascii/{name}/frames` and played client-side.

### Spacer

An invisible layout widget for padding and alignment. Available in three sizes: `1x1`, `2x1`, `1x2`.

## Dashboard Layout

The grid is **5 columns wide** with **130px rows**. It is responsive — at narrower viewports it collapses to 3 and then 2 columns.

By default all breakpoints share one layout (**auto** mode). Switch to **per-breakpoint** mode in settings to configure 5-column, 3-column, and 2-column layouts independently.

## Project Structure

```
app/                    # Entrypoint + HTTP routes
pkg/
  plugins/              # Plugin manager, dashboard rendering
    github/             # GitHub plugin
      templates/        # Widget templates (embedded)
      widgets.go        # Widget implementations
      github.go         # API client
    ascii/              # ASCII animation plugin
      data/             # Embedded animations (meta.json per subdirectory)
      templates/        # Widget template
      ascii.go          # Plugin implementation
    spacer/             # Spacer plugin
  widgets/              # Widget interface + registry
  settings/             # User settings (JSON on disk)
internal/
  cache/                # TTL cache (30-minute default)
ui/
  templates/            # Page templates (dashboard, settings, etc.)
  static/               # CSS, JS, self-hosted fonts
```

## Adding a Plugin

Each plugin is a self-contained package under `pkg/plugins/`. A plugin provides widgets — small dashboard components the user can pin and arrange.

### 1. Create the package

```
pkg/plugins/yourplugin/
  templates/
    summary_small.tmpl
    summary_wide.tmpl
  widgets.go
```

### 2. Implement the Widget interface

Every widget implements `widgets.Widget`:

```go
type Widget interface {
    Definition() WidgetDef    // Static metadata: ID, name, sizes
    Render(ctx context.Context, filter string, sizeName string) (template.HTML, error)
}
```

Widgets own their templates. Embed them with `//go:embed` and parse once at package init:

```go
//go:embed templates/*.tmpl
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))
```

`Render` fetches data and executes the template — the caller just gets back HTML. See `pkg/plugins/github/widgets.go` for working examples.

### 3. Register it

In `pkg/plugins/plugins.go`, add your widget(s) to the registry inside `NewPluginManager`:

```go
reg.Register(yourplugin.NewSummaryWidget(client))
```

That's it. The dashboard, widget picker, and preview API all work off the registry automatically.

### Templates

Widget templates are plain Go HTML templates. Wrap content in `.widget-fill` to fill the widget card:

```html
<div class="widget-fill">
    <span class="stat-num">{{.Count}}</span>
    <span class="stat-label">something</span>
</div>
```

Each size gets its own template file. Name them however you want — there's no naming convention to follow. Just reference the right filename in your `Render` method.

### Size Options

Sizes define how many grid columns/rows a widget spans:

```go
Sizes: []widgets.SizeOption{
    {Name: "small", W: 1, H: 1},   // 1 column, 1 row
    {Name: "wide",  W: 2, H: 1},   // 2 columns, 1 row
    {Name: "tall",  W: 1, H: 2},   // 1 column, 2 rows
}
```

The grid is 5 columns wide with 130px rows.
