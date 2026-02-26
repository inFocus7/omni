package ui

import (
	"path/filepath"
	"text/template"

	"github.com/infocus7/dashie/pkg/plugins"
)

var templatesDir = filepath.Join("ui", "templates")

func Dashboard(pluginManager *plugins.PluginManager) (*template.Template, error) {
	t, err := template.ParseFiles(filepath.Join(templatesDir, "dashboard.tmpl"))
	if err != nil {
		return nil, err
	}

	return t, nil
}
