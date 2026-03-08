package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	GitHub    GitHubSettings    `json:"github"`
	Dashboard DashboardSettings `json:"dashboard"`
}

type GitHubSettings struct {
	Watched []string `json:"watched"`
}

type DashboardSettings struct {
	Widgets []DashboardWidget `json:"widgets"`
}

type DashboardWidget struct {
	ID       string `json:"id"`
	SizeName string `json:"size_name"`
	Position int    `json:"position"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dash", "settings.json"), nil
}

func Load() (*Settings, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Settings{}, nil
	}
	if err != nil {
		return nil, err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Settings) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NormalizeEntry converts user input to a GitHub Search qualifier:
//   - "myorg/*" or "myorg/" -> "org:myorg"
//   - "myorg/repo"          -> "repo:myorg/repo"
//   - already qualified     -> returned as-is
func NormalizeEntry(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "org:") || strings.HasPrefix(raw, "repo:") {
		return raw
	}
	if strings.HasSuffix(raw, "/*") {
		return "org:" + strings.TrimSuffix(raw, "/*")
	}
	if strings.HasSuffix(raw, "/") {
		return "org:" + strings.TrimSuffix(raw, "/")
	}
	if strings.Contains(raw, "/") {
		return "repo:" + raw
	}
	return "org:" + raw
}
