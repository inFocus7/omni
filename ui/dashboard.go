package ui

import (
	"path/filepath"
	"text/template"
)

var templatesDir = filepath.Join("ui", "templates")

func Dashboard() (*template.Template, error) {
	t, err := template.ParseGlob(filepath.Join(templatesDir, "*.tmpl"))
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
