package store

import (
	"context"
	"errors"
)

// ErrReadOnly is returned by read-only backends on mutation operations.
var ErrReadOnly = errors.New("store is read-only")

// ErrNotFound is returned when an animation or variant does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidInput is returned when palette or frame content fails validation.
var ErrInvalidInput = errors.New("invalid input")

// AnimationMeta contains metadata about an animation (without frame data).
type AnimationMeta struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Author   string        `json:"author,omitempty"`
	Source   string        `json:"source,omitempty"`
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
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Author   string           `json:"author,omitempty"`
	PackID   string           `json:"pack_id,omitempty"`
	PackName string           `json:"pack_name,omitempty"`
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
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	PackID     string            `json:"pack_id,omitempty"`
	Source     string            `json:"source,omitempty"`
	Size       string            `json:"size"`
	Cols       int               `json:"cols"`
	Rows       int               `json:"rows"`
	FPS        int               `json:"fps"`
	Palette    map[string]string `json:"palette,omitempty"`
	FirstFrame string            `json:"-"` // frame[0] as plain HTML; used by Render()
	FramesGzip []byte            `json:"-"` // gzip-compressed ICG JSON blob (ICGData)
}

// EventKind identifies the type of store event.
type EventKind int

const (
	EventPut    EventKind = iota // variant created or updated
	EventDelete                  // animation deleted
)

// Event carries a store change notification.
type Event struct {
	Kind        EventKind
	Variant     AnimationVariant
	Name        string
	AnimationID string
	VersionID   string
}

// SummaryPage is the result of a paginated ListSummariesPaged query.
type SummaryPage struct {
	Animations []AnimationSummary `json:"animations"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}

// Pack represents a named collection of animations.
type Pack struct {
	ID          string   `json:"id"`
	Author      string   `json:"author"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	License     string   `json:"license,omitempty"`
	Source      string   `json:"source,omitempty"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
}

// Animation represents an animation identity record in the new schema.
type Animation struct {
	ID       string   `json:"id"`
	Author   string   `json:"author"`
	PackID   string   `json:"pack_id,omitempty"`
	PackName string   `json:"pack_name,omitempty"`
	Name     string   `json:"name"`
	Tags     []string `json:"tags"`
	Source   string   `json:"source,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// VersionMeta describes one version of an animation.
type VersionMeta struct {
	ID          string `json:"id"`
	AnimationID string `json:"animation_id"`
	Version     string `json:"version"`
	CreatedAt   string `json:"created_at"`
}

// SizeVariantData holds data for one size variant within a version.
type SizeVariantData struct {
	Size       string            `json:"size"`
	Cols       int               `json:"cols"`
	Rows       int               `json:"rows"`
	FPS        int               `json:"fps"`
	Palette    map[string]string `json:"palette,omitempty"`
	FirstFrame string            `json:"first_frame,omitempty"`
	FramesGzip []byte            `json:"-"`
}

// AnimationDetail is a fully-loaded animation with version and size variant metadata.
type AnimationDetail struct {
	Animation
	Version      VersionMeta       `json:"version"`
	SizeVariants []SizeVariantData `json:"size_variants"`
}

// AnimationFilters holds filter parameters for listing animations.
type AnimationFilters struct {
	Author string
	PackID string
	Tags   []string
	Query  string
}

// SummaryFilters holds filter and pagination parameters for summary queries.
type SummaryFilters struct {
	Author     string
	PackID     string
	Tags       []string
	Query      string
	SizeFilter string
	Version    string
	Page       int
	PageSize   int
}

// Store is the interface every animation backend must implement.
type Store interface {
	// List returns metadata for all known animations (no frame data).
	List(ctx context.Context) ([]AnimationMeta, error)

	// ListSummaries returns animation metadata with first frames for gallery rendering.
	ListSummaries(ctx context.Context) ([]AnimationSummary, error)

	// ListSummariesPaged returns a page of animation summaries filtered by query and size.
	ListSummariesPaged(ctx context.Context, query, sizeFilter string, page, pageSize int) (SummaryPage, error)

	// ListDistinctSizes returns all distinct variant sizes present in the store.
	ListDistinctSizes(ctx context.Context) ([]string, error)

	// Get returns all loaded variants for the named animation (name or UUID).
	Get(ctx context.Context, name string) ([]AnimationVariant, error)

	// GetVariant returns a specific size variant of an animation.
	GetVariant(ctx context.Context, name, size string) (*AnimationVariant, error)

	// Put creates or replaces a variant. Returns the animation UUID.
	Put(ctx context.Context, variant AnimationVariant) (string, error)

	// Delete removes an animation and all its variants.
	Delete(ctx context.Context, name string) error

	// DeleteVariant removes a single size variant of an animation.
	// Returns ErrNotFound if the variant does not exist.
	DeleteVariant(ctx context.Context, name, size string) error

	// Watch returns a channel that emits events when the store changes.
	// The channel is closed when ctx is cancelled or the store is closed.
	Watch(ctx context.Context) (<-chan Event, error)

	// Close releases resources held by the store.
	Close() error

	// --- Packs ---

	ListPacks(ctx context.Context, author string) ([]Pack, error)
	GetPack(ctx context.Context, id string) (*Pack, error)
	PutPack(ctx context.Context, p Pack) error
	DeletePack(ctx context.Context, id string) error

	// --- Animations (new schema) ---

	ListAnimationsV2(ctx context.Context, filters AnimationFilters) ([]Animation, error)
	GetAnimationByID(ctx context.Context, id string) (*Animation, error)
	GetAnimationByName(ctx context.Context, author, name string) (*Animation, error)
	GetAnimationInPack(ctx context.Context, packID, name string) (*Animation, error)
	PutAnimationV2(ctx context.Context, a Animation) error
	DeleteAnimationByID(ctx context.Context, id string) error

	// --- Versions ---

	ListVersions(ctx context.Context, animationID string) ([]VersionMeta, error)
	GetLatestVersion(ctx context.Context, animationID string) (*VersionMeta, error)
	PutVersion(ctx context.Context, animationID string, version string) (string, error)
	DeleteVersion(ctx context.Context, versionID string) error

	// --- Size variants ---

	GetSizeVariantByVersionID(ctx context.Context, versionID, size string) (*SizeVariantData, error)
	PutSizeVariant(ctx context.Context, versionID string, sv SizeVariantData) error
	ListSizeVariants(ctx context.Context, versionID string) ([]SizeVariantData, error)

	// --- Queries ---

	ListAuthors(ctx context.Context) ([]string, error)
	ListDistinctTagsV2(ctx context.Context) ([]string, error)
}
