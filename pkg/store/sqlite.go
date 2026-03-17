package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a read-write Store backed by a SQLite database.
//
// Schema:
//   - animations: one row per animation name (the identity record)
//   - variants:   metadata per (name, size) — kept lean, no frame data
//   - animation_frames: frame payload per (name, size) — queried only when needed
//
// On Open, if the animations table is empty the embedded fs.FS is used to seed
// default animations via ParseMetaJSON / Put.
//
// Watch uses an in-process broadcast: all mutations flow through Put/Delete in
// the same process, so there is no need for polling or external pub/sub.
type SQLiteStore struct {
	db   *sql.DB
	mu   sync.Mutex
	subs []chan<- Event
}

// OpenSQLite opens (or creates) the SQLite database at path, applies the
// schema, and seeds from seed if the animations table is empty.
//
// Pass an empty path to use a local "dashie.db" file in the working directory.
// Pass ":memory:" for a fully in-memory database (useful for tests).
func OpenSQLite(path string, seed fs.FS) (*SQLiteStore, error) {
	dsn := path
	if dsn == "" {
		dsn = "dashie.db"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}

	// Single writer: limit to one connection so per-connection PRAGMAs
	// (foreign_keys) are always active and no "database is locked" errors occur.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &SQLiteStore{db: db}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}

	if seed != nil {
		if err := s.seedIfEmpty(seed); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite seed: %w", err)
		}
	}

	return s, nil
}

// migrate applies PRAGMAs and creates tables/indices idempotently.
func (s *SQLiteStore) migrate() error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS animations (
			name   TEXT NOT NULL PRIMARY KEY,
			source TEXT                        -- registry URL; NULL for local/built-in
		)`,
		`CREATE TABLE IF NOT EXISTS variants (
			name    TEXT    NOT NULL,
			size    TEXT    NOT NULL,
			cols    INTEGER NOT NULL,
			rows    INTEGER NOT NULL,
			fps     INTEGER NOT NULL,
			palette TEXT    NOT NULL DEFAULT '{}',
			PRIMARY KEY (name, size),
			FOREIGN KEY (name) REFERENCES animations(name) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS animation_frames (
			name   TEXT NOT NULL,
			size   TEXT NOT NULL,
			frames BLOB NOT NULL,
			PRIMARY KEY (name, size),
			FOREIGN KEY (name) REFERENCES animations(name) ON DELETE CASCADE
		)`,
		// Explicit index on variants(name) to make List()'s per-name
		// metadata queries fast without touching animation_frames.
		`CREATE INDEX IF NOT EXISTS idx_variants_name ON variants(name)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(len(stmt), 40)], err)
		}
	}

	// Idempotent column additions for databases created before this field existed.
	if err := s.addColumnIfMissing("animations", "source", "TEXT"); err != nil {
		return err
	}

	return nil
}

// addColumnIfMissing runs ALTER TABLE … ADD COLUMN and silently ignores the
// "duplicate column name" error SQLite returns when the column already exists.
func (s *SQLiteStore) addColumnIfMissing(table, column, colType string) error {
	_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

// seedIfEmpty populates the database from the embedded fs.FS when the
// animations table is empty. It is safe to call on every startup; it is a
// no-op after the first successful seed.
func (s *SQLiteStore) seedIfEmpty(fsys fs.FS) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM animations").Scan(&count); err != nil {
		return fmt.Errorf("count animations: %w", err)
	}
	if count > 0 {
		return nil
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read embedded data dir: %w", err)
	}

	ctx := context.Background()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := e.Name()
		data, err := fs.ReadFile(fsys, filepath.Join(dir, "meta.json"))
		if err != nil {
			continue
		}
		pack, err := ParseMetaJSON(data)
		if err != nil {
			continue
		}
		for _, vs := range pack.Variants {
			framesData, err := fs.ReadFile(fsys, filepath.Join(dir, vs.FramesFile))
			if err != nil {
				return fmt.Errorf("seed %q: read frames %q: %w", pack.Name, vs.FramesFile, err)
			}
			var frames []string
			if err := json.Unmarshal(framesData, &frames); err != nil {
				return fmt.Errorf("seed %q: parse frames %q: %w", pack.Name, vs.FramesFile, err)
			}
			v := AnimationVariant{
				Name:    pack.Name,
				Size:    vs.Size,
				Cols:    vs.Cols,
				Rows:    vs.Rows,
				FPS:     vs.FPS,
				Palette: pack.Palette,
				Frames:  frames,
			}
			if err := s.Put(ctx, v); err != nil {
				return fmt.Errorf("seed %q/%q: %w", pack.Name, vs.Size, err)
			}
		}
	}
	return nil
}

// List returns metadata for all known animations without loading frame data.
func (s *SQLiteStore) List(ctx context.Context) ([]AnimationMeta, error) {
	rs, err := s.db.QueryContext(ctx, `
		SELECT a.name, a.source, v.size, v.cols, v.rows, v.fps
		FROM animations a
		LEFT JOIN variants v ON v.name = a.name
		ORDER BY a.name, v.size
	`)
	if err != nil {
		return nil, fmt.Errorf("list animations: %w", err)
	}
	defer rs.Close()

	var order []string
	byName := map[string]*AnimationMeta{}

	for rs.Next() {
		var name string
		var source sql.NullString
		var size sql.NullString
		var cols, rows, fps sql.NullInt64
		if err := rs.Scan(&name, &source, &size, &cols, &rows, &fps); err != nil {
			return nil, fmt.Errorf("scan animation row: %w", err)
		}
		if _, exists := byName[name]; !exists {
			byName[name] = &AnimationMeta{Name: name, Source: source.String}
			order = append(order, name)
		}
		if size.Valid {
			byName[name].Variants = append(byName[name].Variants, VariantMeta{
				Size: size.String,
				Cols: int(cols.Int64),
				Rows: int(rows.Int64),
				FPS:  int(fps.Int64),
			})
		}
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("iterate animations: %w", err)
	}

	result := make([]AnimationMeta, 0, len(order))
	for _, name := range order {
		result = append(result, *byName[name])
	}
	return result, nil
}

// Get returns all loaded variants (including frames) for the named animation.
func (s *SQLiteStore) Get(ctx context.Context, name string) ([]AnimationVariant, error) {
	rs, err := s.db.QueryContext(ctx, `
		SELECT a.source, v.size, v.cols, v.rows, v.fps, v.palette, f.frames
		FROM variants v
		JOIN animations a ON a.name = v.name
		JOIN animation_frames f ON f.name = v.name AND f.size = v.size
		WHERE v.name = ?
		ORDER BY v.size
	`, name)
	if err != nil {
		return nil, fmt.Errorf("get animation %q: %w", name, err)
	}
	defer rs.Close()

	var variants []AnimationVariant
	for rs.Next() {
		v, err := scanVariant(rs, name)
		if err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("iterate variants for %q: %w", name, err)
	}
	if len(variants) == 0 {
		return nil, ErrNotFound
	}
	return variants, nil
}

// GetVariant returns a specific size variant of an animation.
func (s *SQLiteStore) GetVariant(ctx context.Context, name, size string) (*AnimationVariant, error) {
	rs, err := s.db.QueryContext(ctx, `
		SELECT a.source, v.size, v.cols, v.rows, v.fps, v.palette, f.frames
		FROM variants v
		JOIN animations a ON a.name = v.name
		JOIN animation_frames f ON f.name = v.name AND f.size = v.size
		WHERE v.name = ? AND v.size = ?
	`, name, size)
	if err != nil {
		return nil, fmt.Errorf("get variant %q/%q: %w", name, size, err)
	}
	defer rs.Close()

	if !rs.Next() {
		if err := rs.Err(); err != nil {
			return nil, fmt.Errorf("query variant %q/%q: %w", name, size, err)
		}
		return nil, ErrNotFound
	}
	v, err := scanVariant(rs, name)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Put creates or replaces a variant and its frame data atomically.
func (s *SQLiteStore) Put(ctx context.Context, v AnimationVariant) error {
	paletteJSON, err := json.Marshal(v.Palette)
	if err != nil {
		return fmt.Errorf("marshal palette: %w", err)
	}
	framesBlob, err := json.Marshal(v.Frames)
	if err != nil {
		return fmt.Errorf("marshal frames: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Insert the animation row. If it already exists, only update source when
	// the incoming value is non-empty — registry syncs set it, local API
	// writes and seeding leave any existing source untouched.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO animations(name, source) VALUES(?,?)
		ON CONFLICT(name) DO UPDATE SET
			source = CASE WHEN excluded.source != '' THEN excluded.source ELSE source END
	`, v.Name, v.Source); err != nil {
		return fmt.Errorf("upsert animation: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO variants(name, size, cols, rows, fps, palette)
		 VALUES(?,?,?,?,?,?)`,
		v.Name, v.Size, v.Cols, v.Rows, v.FPS, string(paletteJSON),
	); err != nil {
		return fmt.Errorf("upsert variant: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO animation_frames(name, size, frames) VALUES(?,?,?)`,
		v.Name, v.Size, framesBlob,
	); err != nil {
		return fmt.Errorf("upsert frames: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.broadcast(Event{Kind: EventPut, Variant: v, Name: v.Name})
	return nil
}

// Delete removes an animation and all its variants (cascade).
func (s *SQLiteStore) Delete(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM animations WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete animation %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.broadcast(Event{Kind: EventDelete, Name: name})
	return nil
}

// Watch returns a channel that emits events when animations are created,
// updated, or deleted. The channel is closed when ctx is cancelled.
func (s *SQLiteStore) Watch(ctx context.Context) (<-chan Event, error) {
	ch := make(chan Event, 64)
	s.mu.Lock()
	s.subs = append(s.subs, ch)
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, sub := range s.subs {
			if sub == ch {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				break
			}
		}
		close(ch)
	}()

	return ch, nil
}

// Close releases all resources. Open Watch channels are closed.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	subs := s.subs
	s.subs = nil
	s.mu.Unlock()
	for _, sub := range subs {
		close(sub)
	}
	return s.db.Close()
}

// broadcast sends ev to all active Watch subscribers, dropping if a
// subscriber's buffer is full (consistent with prior K8sStore behaviour).
func (s *SQLiteStore) broadcast(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		select {
		case sub <- ev:
		default:
		}
	}
}

// scanVariant reads one row from a query that selects
// (source TEXT, size, cols, rows, fps, palette TEXT, frames BLOB).
func scanVariant(rs *sql.Rows, name string) (AnimationVariant, error) {
	var source sql.NullString
	var size, paletteJSON string
	var cols, rows, fps int
	var framesBlob []byte

	if err := rs.Scan(&source, &size, &cols, &rows, &fps, &paletteJSON, &framesBlob); err != nil {
		return AnimationVariant{}, fmt.Errorf("scan variant row: %w", err)
	}

	var palette map[string]string
	if err := json.Unmarshal([]byte(paletteJSON), &palette); err != nil {
		palette = nil
	}

	var frames []string
	if err := json.Unmarshal(framesBlob, &frames); err != nil {
		return AnimationVariant{}, fmt.Errorf("unmarshal frames for %q/%q: %w", name, size, err)
	}

	return AnimationVariant{
		Name:    name,
		Source:  source.String,
		Size:    size,
		Cols:    cols,
		Rows:    rows,
		FPS:     fps,
		Palette: palette,
		Frames:  frames,
	}, nil
}

