package store

import (
	"context"
	"errors"
)

// ErrReadOnly is returned by read-only backends on mutation operations.
var ErrReadOnly = errors.New("store is read-only")

// ErrNotFound is returned when an animation or variant does not exist.
var ErrNotFound = errors.New("not found")

// AnimationMeta contains metadata about an animation (without frame data).
type AnimationMeta struct {
	Name     string        `json:"name"`
	Source   string        `json:"source,omitempty"` // registry URL, empty for local/built-in
	Variants []VariantMeta `json:"variants"`
}

// VariantMeta is the lightweight descriptor for one size variant.
type VariantMeta struct {
	Size string `json:"size"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	FPS  int    `json:"fps"`
}

// AnimationSummary groups animation metadata with first-frame thumbnails for gallery rendering.
type AnimationSummary struct {
	Name     string           `json:"name"`
	Source   string           `json:"source,omitempty"`
	Variants []VariantSummary `json:"variants"`
}

// VariantSummary is a lightweight descriptor for one size variant, including
// the first frame for server-rendered thumbnails.
type VariantSummary struct {
	Size       string            `json:"size"`
	Cols       int               `json:"cols"`
	Rows       int               `json:"rows"`
	FPS        int               `json:"fps"`
	Palette    map[string]string `json:"palette,omitempty"`
	FirstFrame string            `json:"first_frame"`
}

// AnimationVariant is a fully-loaded variant, including frame data.
type AnimationVariant struct {
	Name       string            `json:"name"`
	Source     string            `json:"source,omitempty"`
	Size       string            `json:"size"`
	Cols       int               `json:"cols"`
	Rows       int               `json:"rows"`
	FPS        int               `json:"fps"`
	Palette    map[string]string `json:"palette,omitempty"`
	FirstFrame string            `json:"-"` // frame[0] as plain HTML; used by Render()
	FramesGzip []byte            `json:"-"` // gzip-compressed JSON []string; opaque blob
}

// EventKind identifies the type of store event.
type EventKind int

const (
	EventPut    EventKind = iota // variant created or updated
	EventDelete                  // animation deleted
)

// Event carries a store change notification.
type Event struct {
	Kind    EventKind
	Variant AnimationVariant
	Name    string
}

// Store is the interface every animation backend must implement.
type Store interface {
	// List returns metadata for all known animations (no frame data).
	List(ctx context.Context) ([]AnimationMeta, error)

	// ListSummaries returns animation metadata with first frames for gallery rendering.
	ListSummaries(ctx context.Context) ([]AnimationSummary, error)

	// Get returns all loaded variants for the named animation.
	Get(ctx context.Context, name string) ([]AnimationVariant, error)

	// GetVariant returns a specific size variant of an animation.
	GetVariant(ctx context.Context, name, size string) (*AnimationVariant, error)

	// Put creates or replaces a variant.
	Put(ctx context.Context, variant AnimationVariant) error

	// Delete removes an animation and all its variants.
	Delete(ctx context.Context, name string) error

	// Watch returns a channel that emits events when the store changes.
	// The channel is closed when ctx is cancelled or the store is closed.
	Watch(ctx context.Context) (<-chan Event, error)

	// Close releases resources held by the store.
	Close() error
}
