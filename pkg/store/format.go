package store

import (
	"encoding/json"
	"fmt"
)

// PackMeta is the parsed content of a pack-level meta.json.
// It describes an animation and lists its size variants with references to
// separate frames files rather than inlining the frame data.
type PackMeta struct {
	Name     string            `json:"name"`
	Palette  map[string]string `json:"palette,omitempty"`
	Variants []VariantFileMeta `json:"variants"`
}

// VariantFileMeta describes one size variant and where to find its frames.
// FramesFile is a filename relative to the animation's directory.
type VariantFileMeta struct {
	Size       string `json:"size"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
	FPS        int    `json:"fps"`
	FramesFile string `json:"frames_file"`
}

// ParseMetaJSON parses a meta.json byte slice into a PackMeta.
func ParseMetaJSON(data []byte) (PackMeta, error) {
	var m PackMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return PackMeta{}, fmt.Errorf("parse meta.json: %w", err)
	}
	if m.Name == "" {
		return PackMeta{}, fmt.Errorf("meta.json: missing name field")
	}
	if len(m.Variants) == 0 {
		return PackMeta{}, fmt.Errorf("meta.json: no variants defined")
	}
	for i, v := range m.Variants {
		if v.Size == "" {
			return PackMeta{}, fmt.Errorf("meta.json: variant %d missing size", i)
		}
		if v.FramesFile == "" {
			return PackMeta{}, fmt.Errorf("meta.json: variant %q missing frames_file", v.Size)
		}
	}
	return m, nil
}
