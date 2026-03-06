package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/google/go-github/v83/github"
)

var templatesDir = filepath.Join("ui", "templates")

func funcMap() template.FuncMap {
	return template.FuncMap{
		// repoFromURL extracts "owner/repo" from a GitHub API repository URL.
		// e.g. "https://api.github.com/repos/acme/myrepo" → "acme/myrepo"
		"repoFromURL": func(url string) string {
			parts := strings.Split(url, "/repos/")
			if len(parts) < 2 {
				return url
			}
			return parts[1]
		},

		// repoHTMLURL converts a GitHub API repository URL to its web URL.
		// e.g. "https://api.github.com/repos/acme/myrepo" → "https://github.com/acme/myrepo"
		"repoHTMLURL": func(url string) string {
			parts := strings.Split(url, "/repos/")
			if len(parts) < 2 {
				return ""
			}
			return "https://github.com/" + parts[1]
		},

		// repoAvatarURL returns the avatar URL for the repo owner (user or org).
		// e.g. "https://api.github.com/repos/acme/myrepo" → "https://avatars.githubusercontent.com/acme?s=20"
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
	}
}

// Pages loads all templates (dashboard + plugin pages) into a shared set with helper functions.
func Pages() (*template.Template, error) {
	t, err := template.New("root").Funcs(funcMap()).ParseGlob(filepath.Join(templatesDir, "*.tmpl"))
	if err != nil {
		return nil, err
	}

	pluginMatches, err := filepath.Glob(filepath.Join(templatesDir, "plugins", "*.tmpl"))
	if err != nil {
		return nil, err
	}

	if len(pluginMatches) > 0 {
		t, err = t.ParseFiles(pluginMatches...)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}
