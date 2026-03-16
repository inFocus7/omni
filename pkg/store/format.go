package store

import (
	"encoding/json"
	"fmt"
)

// metaJSON is the on-disk JSON format for a single animation variant.
type metaJSON struct {
	Name    string            `json:"name"`
	Size    string            `json:"size"`
	Cols    int               `json:"cols"`
	Rows    int               `json:"rows"`
	FPS     int               `json:"fps"`
	Palette map[string]string `json:"palette,omitempty"` // class name → CSS colour
	Frames  []string          `json:"frames"`
}

// ParseMetaJSON parses a meta.json byte slice into an AnimationVariant.
func ParseMetaJSON(data []byte) (AnimationVariant, error) {
	var m metaJSON
	if err := json.Unmarshal(data, &m); err != nil {
		return AnimationVariant{}, fmt.Errorf("parse meta.json: %w", err)
	}
	if m.Name == "" {
		return AnimationVariant{}, fmt.Errorf("meta.json: missing name field")
	}
	if m.Size == "" {
		m.Size = "1x1"
	}
	return AnimationVariant{
		Name:    m.Name,
		Size:    m.Size,
		Cols:    m.Cols,
		Rows:    m.Rows,
		FPS:     m.FPS,
		Palette: m.Palette,
		Frames:  m.Frames,
	}, nil
}

// MarshalVariant serialises an AnimationVariant to meta.json bytes.
func MarshalVariant(v AnimationVariant) ([]byte, error) {
	m := metaJSON{
		Name:    v.Name,
		Size:    v.Size,
		Cols:    v.Cols,
		Rows:    v.Rows,
		FPS:     v.FPS,
		Palette: v.Palette,
		Frames:  v.Frames,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal meta.json: %w", err)
	}
	return data, nil
}
