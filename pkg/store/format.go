package store

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
)

// GzipCompress compresses data using gzip and returns the compressed bytes.
func GzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GzipDecompress decompresses gzip-compressed data.
func GzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// PackMeta is the parsed content of a pack-level meta.json.
// It describes an animation and lists its size variants with references to
// separate frames files rather than inlining the frame data.
type PackMeta struct {
	Name     string            `json:"name"`
	Version  string            `json:"version,omitempty"`
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

// CompressFrames marshals frames to JSON, gzip-compresses the result, and
// returns the compressed blob along with the first frame as a plain string.
// Returns an error if frames is empty or compression fails.
func CompressFrames(frames []string) (gz []byte, firstFrame string, err error) {
	if len(frames) == 0 {
		return nil, "", fmt.Errorf("CompressFrames: frames must not be empty")
	}
	data, err := json.Marshal(frames)
	if err != nil {
		return nil, "", fmt.Errorf("CompressFrames: marshal: %w", err)
	}
	gz, err = GzipCompress(data)
	if err != nil {
		return nil, "", fmt.Errorf("CompressFrames: compress: %w", err)
	}
	return gz, frames[0], nil
}

// PackJSON is the parsed content of a pack.json file that describes a
// multi-animation bundle. It lists animation subdirectory names and optional
// metadata fields used when generating a pack for export.
type PackJSON struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	License     string   `json:"license"`
	Animations  []string `json:"animations"`
}

// ParsePackJSON parses a pack.json byte slice into a PackJSON.
// Returns an error if the animations list is empty.
func ParsePackJSON(data []byte) (PackJSON, error) {
	var p PackJSON
	if err := json.Unmarshal(data, &p); err != nil {
		return PackJSON{}, fmt.Errorf("parse pack.json: %w", err)
	}
	if len(p.Animations) == 0 {
		return PackJSON{}, fmt.Errorf("pack.json: animations list must not be empty")
	}
	return p, nil
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
