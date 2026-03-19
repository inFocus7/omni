package ui

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v83/github"
)

var templatesDir = filepath.Join("ui", "templates")

func funcMap() template.FuncMap {
	return template.FuncMap{
		// repoFromURL extracts "owner/repo" from a GitHub API repository URL.
		// e.g. "https://api.github.com/repos/acme/myrepo" -> "acme/myrepo"
		"repoFromURL": func(url string) string {
			parts := strings.Split(url, "/repos/")
			if len(parts) < 2 {
				return url
			}
			return parts[1]
		},

		// repoHTMLURL converts a GitHub API repository URL to its web URL.
		// e.g. "https://api.github.com/repos/acme/myrepo" -> "https://github.com/acme/myrepo"
		"repoHTMLURL": func(url string) string {
			parts := strings.Split(url, "/repos/")
			if len(parts) < 2 {
				return ""
			}
			return "https://github.com/" + parts[1]
		},

		// repoAvatarURL returns the avatar URL for the repo owner (user or org).
		// e.g. "https://api.github.com/repos/acme/myrepo" -> "https://avatars.githubusercontent.com/acme?s=20"
		"repoAvatarURL": func(url string) string {
			parts := strings.Split(url, "/repos/")
			if len(parts) < 2 {
				return ""
			}
			owner := strings.SplitN(parts[1], "/", 2)[0]
			return "https://avatars.githubusercontent.com/" + owner + "?s=40"
		},

		// ageStr returns a human-readable age string like "3d", "2w", "1m".
		"ageStr": func(t *github.Timestamp) string {
			if t == nil {
				return "—"
			}
			d := time.Since(t.Time)
			switch {
			case d < time.Minute:
				return "just now"
			case d < time.Hour:
				return fmt.Sprintf("%dm", int(d.Minutes()))
			case d < 24*time.Hour:
				return fmt.Sprintf("%dh", int(d.Hours()))
			case d < 7*24*time.Hour:
				return fmt.Sprintf("%dd", int(d.Hours()/24))
			case d < 30*24*time.Hour:
				return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
			default:
				return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
			}
		},

		// ageUnix returns the Unix timestamp for a GitHub timestamp, used as a sort key.
		"ageUnix": func(t *github.Timestamp) int64 {
			if t == nil {
				return 0
			}
			return t.Time.Unix()
		},

		// isDraft safely dereferences a *bool draft field.
		"isDraft": func(b *bool) bool {
			return b != nil && *b
		},

		// watchedName strips the qualifier prefix from a watched entry.
		// "org:myorg" → "myorg", "repo:owner/repo" -> "owner/repo"
		"watchedName": func(s string) string {
			if after, ok := strings.CutPrefix(s, "org:"); ok {
				return after
			}
			if after, ok := strings.CutPrefix(s, "repo:"); ok {
				return after
			}
			return s
		},

		// avatarSize appends a size parameter to a GitHub avatar URL.
		// e.g. "https://avatars.githubusercontent.com/u/123?v=4" -> "...?v=4&s=40"
		"avatarSize": func(url string, size int) string {
			if strings.Contains(url, "?") {
				return fmt.Sprintf("%s&s=%d", url, size)
			}
			return fmt.Sprintf("%s?s=%d", url, size)
		},

		// watchedType returns "org" or "repo" for a watched entry qualifier.
		"watchedType": func(s string) string {
			if strings.HasPrefix(s, "org:") {
				return "org"
			}
			return "repo"
		},

		// sizeW / sizeH parse the W and H components from a "WxH" size string.
		"sizeW": func(s string) int {
			parts := strings.SplitN(s, "x", 2)
			if len(parts) != 2 {
				return 1
			}
			n, _ := strconv.Atoi(parts[0])
			if n <= 0 {
				return 1
			}
			return n
		},
		"sizeH": func(s string) int {
			parts := strings.SplitN(s, "x", 2)
			if len(parts) != 2 {
				return 1
			}
			n, _ := strconv.Atoi(parts[1])
			if n <= 0 {
				return 1
			}
			return n
		},

		// safeHTML marks a string as safe HTML, bypassing template escaping.
		// Only use for trusted content stored in the database.
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},

		// paletteCSS builds a container-scoped <style> block from a map of CSS class names to colors.
		// name and size identify the animation variant and are used to construct the scope ID
		// "asc-{name}-{size}", so rules are "#asc-name-size .class{color:value}".
		"paletteCSS": func(name, size string, palette map[string]string) template.HTML {
			if len(palette) == 0 {
				return ""
			}
			scope := "asc-" + name + "-" + size
			var b strings.Builder
			b.WriteString("<style>")
			for class, color := range palette {
				fmt.Fprintf(&b, "#%s .%s{color:%s}", scope, template.HTMLEscapeString(class), template.HTMLEscapeString(color))
			}
			b.WriteString("</style>")
			return template.HTML(b.String())
		},
	}
}

// Pages loads all page templates, each combined with the shared base layout and plugin sub-templates.
func Pages() (map[string]*template.Template, error) {
	base := filepath.Join(templatesDir, "base.tmpl")

	pluginMatches, err := filepath.Glob(filepath.Join(templatesDir, "plugins", "*.tmpl"))
	if err != nil {
		return nil, err
	}

	pageMatches, err := filepath.Glob(filepath.Join(templatesDir, "*.tmpl"))
	if err != nil {
		return nil, err
	}

	pages := make(map[string]*template.Template, len(pageMatches))
	for _, pf := range pageMatches {
		name := filepath.Base(pf)
		if name == "base.tmpl" {
			continue
		}

		files := []string{base, pf}
		files = append(files, pluginMatches...)

		t, err := template.New("").Funcs(funcMap()).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		pages[name] = t
	}

	return pages, nil
}
