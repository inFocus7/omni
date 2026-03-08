# OMNI

A developer dashboard. Primarily made for myself to track my contribution ratio (PR:Review ratio).

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

## Project Structure

```
app/                    # Entrypoint + HTTP routes
pkg/
  plugins/              # Plugin manager, dashboard rendering
    github/             # GitHub plugin
      templates/        # Widget templates (embedded)
      widgets.go        # Widget implementations
      github.go         # API client
  widgets/              # Widget interface + registry
  settings/             # User settings (JSON on disk)
ui/
  templates/            # Page templates (dashboard, settings, etc.)
  static/               # CSS + JS
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
