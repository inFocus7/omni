package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a read-write Store backed by a SQLite database.
type SQLiteStore struct {
	db   *sql.DB
	mu   sync.Mutex
	subs []chan<- Event
}

// NewUUID generates a random UUID v4.
func NewUUID() string { return newUUID() }

// newUUID generates a random UUID v4.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// OpenSQLite opens (or creates) the SQLite database at path, applies the
// schema, and seeds from seed if the animations table is empty.
func OpenSQLite(path string, seed fs.FS) (*SQLiteStore, error) {
	dsn := path
	if dsn == "" {
		dsn = "dashie.db"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &SQLiteStore{db: db}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}

	if err := s.migrateV1toV2(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate v1→v2: %w", err)
	}

	if err := s.migrateHTMLToICG(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate HTML→ICG: %w", err)
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
			source TEXT
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
		`CREATE INDEX IF NOT EXISTS idx_variants_name ON variants(name)`,
		`CREATE TABLE IF NOT EXISTS packs (
			id          TEXT NOT NULL PRIMARY KEY,
			author      TEXT NOT NULL,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			license     TEXT NOT NULL DEFAULT '',
			source      TEXT,
			tags        TEXT NOT NULL DEFAULT '[]',
			created_at  TEXT NOT NULL,
			UNIQUE(author, name)
		)`,
		`CREATE TABLE IF NOT EXISTS animations_v2 (
			id         TEXT NOT NULL PRIMARY KEY,
			author     TEXT NOT NULL DEFAULT 'user',
			pack_id    TEXT REFERENCES packs(id) ON DELETE SET NULL,
			name       TEXT NOT NULL,
			tags       TEXT NOT NULL DEFAULT '[]',
			source     TEXT,
			created_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_anim_ungrouped ON animations_v2(author, name) WHERE pack_id IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_anim_grouped ON animations_v2(pack_id, name) WHERE pack_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_anim_author ON animations_v2(author)`,
		`CREATE INDEX IF NOT EXISTS idx_anim_pack ON animations_v2(pack_id)`,
		`CREATE TABLE IF NOT EXISTS animation_versions (
			id           TEXT NOT NULL PRIMARY KEY,
			animation_id TEXT NOT NULL REFERENCES animations_v2(id) ON DELETE CASCADE,
			version      TEXT NOT NULL,
			created_at   TEXT NOT NULL,
			UNIQUE(animation_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ver_animation ON animation_versions(animation_id)`,
		`CREATE TABLE IF NOT EXISTS size_variants (
			version_id  TEXT NOT NULL REFERENCES animation_versions(id) ON DELETE CASCADE,
			size        TEXT NOT NULL,
			cols        INTEGER NOT NULL,
			rows        INTEGER NOT NULL,
			fps         INTEGER NOT NULL,
			palette     TEXT NOT NULL DEFAULT '{}',
			frames      BLOB NOT NULL,
			first_frame TEXT NOT NULL DEFAULT '',
			PRIMARY KEY(version_id, size)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sv_version ON size_variants(version_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(len(stmt), 40)], err)
		}
	}

	if err := s.addColumnIfMissing("animations", "source", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("animation_frames", "first_frame", "TEXT NOT NULL DEFAULT ''"); err != nil {
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

// migrateV1toV2 migrates from the old schema (animations/variants/animation_frames)
// to the new schema (animations_v2/animation_versions/size_variants), then renames
// animations_v2 to animations.
func (s *SQLiteStore) migrateV1toV2() error {
	// Check if old variants table exists.
	var hasOldVariants int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='variants'`).Scan(&hasOldVariants); err != nil {
		return fmt.Errorf("check old schema: %w", err)
	}
	if hasOldVariants == 0 {
		return nil
	}

	// Check if animations_v2 is already populated.
	var v2count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM animations_v2`).Scan(&v2count); err != nil {
		return fmt.Errorf("count animations_v2: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	nameToUUID := map[string]string{}

	if v2count == 0 {
		// Migrate old animations rows.
		animRows, err := s.db.Query(`SELECT name, source FROM animations`)
		if err != nil {
			return fmt.Errorf("read old animations: %w", err)
		}
		type oldAnim struct {
			name, source string
		}
		var oldAnims []oldAnim
		for animRows.Next() {
			var name string
			var source sql.NullString
			if err := animRows.Scan(&name, &source); err != nil {
				animRows.Close()
				return fmt.Errorf("scan old animation: %w", err)
			}
			oldAnims = append(oldAnims, oldAnim{name, source.String})
		}
		animRows.Close()

		for _, a := range oldAnims {
			id := newUUID()
			nameToUUID[a.name] = id
			var src interface{} = nil
			if a.source != "" {
				src = a.source
			}
			if _, err := s.db.Exec(
				`INSERT OR IGNORE INTO animations_v2(id, author, name, source, tags, created_at) VALUES(?,?,?,?,?,?)`,
				id, "user", a.name, src, "[]", now,
			); err != nil {
				return fmt.Errorf("migrate animation %q: %w", a.name, err)
			}
			// Re-read to get actual id in case of conflict.
			var actualID string
			if err := s.db.QueryRow(`SELECT id FROM animations_v2 WHERE author='user' AND name=? AND pack_id IS NULL`, a.name).Scan(&actualID); err == nil {
				nameToUUID[a.name] = actualID
			}
		}

		// Migrate variants + frames.
		varRows, err := s.db.Query(`
			SELECT v.name, v.size, v.cols, v.rows, v.fps, v.palette, f.frames, f.first_frame
			FROM variants v
			LEFT JOIN animation_frames f ON f.name = v.name AND f.size = v.size
		`)
		if err != nil {
			return fmt.Errorf("read old variants: %w", err)
		}
		type oldVariant struct {
			name, size, paletteJSON, firstFrame string
			cols, rows, fps                     int
			frames                              []byte
		}
		var oldVariants []oldVariant
		for varRows.Next() {
			var name, size, paletteJSON string
			var firstFrame sql.NullString
			var cols, rows, fps int
			var frames []byte
			if err := varRows.Scan(&name, &size, &cols, &rows, &fps, &paletteJSON, &frames, &firstFrame); err != nil {
				varRows.Close()
				return fmt.Errorf("scan old variant: %w", err)
			}
			oldVariants = append(oldVariants, oldVariant{name, size, paletteJSON, firstFrame.String, cols, rows, fps, frames})
		}
		varRows.Close()

		verIDs := map[string]string{} // name → verID
		for _, v := range oldVariants {
			animID, ok := nameToUUID[v.name]
			if !ok {
				continue
			}
			verID, exists := verIDs[v.name]
			if !exists {
				verID = newUUID()
				if _, err := s.db.Exec(
					`INSERT OR IGNORE INTO animation_versions(id, animation_id, version, created_at) VALUES(?,?,?,?)`,
					verID, animID, "1.0.0", now,
				); err != nil {
					return fmt.Errorf("migrate version for %q: %w", v.name, err)
				}
				var actualVerID string
				if err := s.db.QueryRow(`SELECT id FROM animation_versions WHERE animation_id=? AND version='1.0.0'`, animID).Scan(&actualVerID); err == nil {
					verID = actualVerID
				}
				verIDs[v.name] = verID
			}
			if len(v.frames) == 0 {
				continue
			}
			if _, err := s.db.Exec(
				`INSERT OR REPLACE INTO size_variants(version_id, size, cols, rows, fps, palette, frames, first_frame) VALUES(?,?,?,?,?,?,?,?)`,
				verID, v.size, v.cols, v.rows, v.fps, v.paletteJSON, v.frames, v.firstFrame,
			); err != nil {
				return fmt.Errorf("migrate size_variant %q/%q: %w", v.name, v.size, err)
			}
		}
	}

	// Migrate settings widget IDs.
	if len(nameToUUID) > 0 {
		s.migrateSettingsWidgetIDs(nameToUUID) //nolint:errcheck
	}

	// Drop old tables and rename animations_v2 → animations.
	dropStmts := []string{
		`DROP TABLE IF EXISTS animation_frames`,
		`DROP TABLE IF EXISTS variants`,
		`DROP TABLE IF EXISTS animations`,
		`ALTER TABLE animations_v2 RENAME TO animations`,
	}
	for _, stmt := range dropStmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("v1→v2 cleanup %q: %w", stmt[:min(len(stmt), 40)], err)
		}
	}

	return nil
}

// migrateSettingsWidgetIDs rewrites "ascii-{name}" widget IDs to "ascii-{uuid}" in settings.json.
func (s *SQLiteStore) migrateSettingsWidgetIDs(nameToUUID map[string]string) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	settingsPath := filepath.Join(dir, "omni", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	rewriteWidgets := func(widgets []interface{}) {
		for _, w := range widgets {
			if wm, ok := w.(map[string]interface{}); ok {
				if id, ok := wm["id"].(string); ok && strings.HasPrefix(id, "ascii-") {
					name := strings.TrimPrefix(id, "ascii-")
					if uuid, ok := nameToUUID[name]; ok {
						wm["id"] = "ascii-" + uuid
					}
				}
			}
		}
	}

	if dashboard, ok := settings["dashboard"].(map[string]interface{}); ok {
		if widgets, ok := dashboard["widgets"].([]interface{}); ok {
			rewriteWidgets(widgets)
		}
		if layouts, ok := dashboard["layouts"].(map[string]interface{}); ok {
			for _, v := range layouts {
				if widgets, ok := v.([]interface{}); ok {
					rewriteWidgets(widgets)
				}
			}
		}
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil
	}
	tmp := settingsPath + ".tmp"
	os.WriteFile(tmp, out, 0644) //nolint:errcheck
	os.Rename(tmp, settingsPath) //nolint:errcheck
	return nil
}

// migrateHTMLToICG detects animation_frames rows still in legacy HTML format
// and converts them to ICG format in-place.
func (s *SQLiteStore) migrateHTMLToICG() error {
	// Skip if on new schema (size_variants exists, animation_frames doesn't).
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='size_variants'`).Scan(&count); err == nil && count > 0 {
		return nil
	}

	rows, err := s.db.Query(`
		SELECT f.name, f.size, f.frames, v.cols, v.rows, v.palette
		FROM animation_frames f
		JOIN variants v ON v.name = f.name AND v.size = f.size
	`)
	if err != nil {
		return fmt.Errorf("query frames for migration: %w", err)
	}
	defer rows.Close()

	type migrationRow struct {
		name, size string
		framesGz   []byte
		cols, rws  int
		palette    map[string]string
	}
	var toMigrate []migrationRow

	for rows.Next() {
		var name, size, paletteJSON string
		var framesGz []byte
		var cols, rws int
		if err := rows.Scan(&name, &size, &framesGz, &cols, &rws, &paletteJSON); err != nil {
			return fmt.Errorf("scan migration row: %w", err)
		}

		plain, err := GzipDecompress(framesGz)
		if err != nil {
			continue
		}
		if IsICGFormat(plain) {
			continue
		}

		var palette map[string]string
		if err := json.Unmarshal([]byte(paletteJSON), &palette); err != nil {
			palette = nil
		}

		toMigrate = append(toMigrate, migrationRow{
			name: name, size: size, framesGz: framesGz,
			cols: cols, rws: rws, palette: palette,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migration rows: %w", err)
	}
	rows.Close()

	for _, mr := range toMigrate {
		plain, _ := GzipDecompress(mr.framesGz)
		var htmlFrames []string
		if err := json.Unmarshal(plain, &htmlFrames); err != nil {
			continue
		}
		icg, err := HTMLFramesToICG(htmlFrames, mr.cols, mr.rws)
		if err != nil {
			continue
		}
		gz, firstFrame, err := CompressICG(icg)
		if err != nil {
			continue
		}
		if _, err := s.db.Exec(
			`UPDATE animation_frames SET frames = ?, first_frame = ? WHERE name = ? AND size = ?`,
			gz, firstFrame, mr.name, mr.size,
		); err != nil {
			return fmt.Errorf("migrate %q/%q: %w", mr.name, mr.size, err)
		}
	}

	return nil
}

// seedIfEmpty populates the database from the embedded fs.FS when the
// animations table is empty.
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
	now := time.Now().UTC().Format(time.RFC3339)

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

		animID := newUUID()
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO animations(id, author, name, source, tags, created_at) VALUES(?,?,?,?,?,?)`,
			animID, "omni", pack.Name, "", "[]", now,
		); err != nil {
			return fmt.Errorf("seed animation %q: %w", pack.Name, err)
		}
		var actualAnimID string
		if err := s.db.QueryRowContext(ctx,
			`SELECT id FROM animations WHERE author='omni' AND name=? AND pack_id IS NULL`, pack.Name,
		).Scan(&actualAnimID); err == nil {
			animID = actualAnimID
		}

		version := "1.0.0"
		if pack.Version != "" {
			version = pack.Version
		}

		verID := newUUID()
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO animation_versions(id, animation_id, version, created_at) VALUES(?,?,?,?)`,
			verID, animID, version, now,
		); err != nil {
			return fmt.Errorf("seed version for %q: %w", pack.Name, err)
		}
		var actualVerID string
		if err := s.db.QueryRowContext(ctx,
			`SELECT id FROM animation_versions WHERE animation_id=? AND version=?`, animID, version,
		).Scan(&actualVerID); err == nil {
			verID = actualVerID
		}

		for _, vs := range pack.Variants {
			framesData, err := fs.ReadFile(fsys, filepath.Join(dir, vs.FramesFile))
			if err != nil {
				return fmt.Errorf("seed %q: read frames %q: %w", pack.Name, vs.FramesFile, err)
			}
			var icg *ICGData
			if IsICGFormat(framesData) {
				icg, err = ParseICGFramesFile(framesData)
				if err != nil {
					return fmt.Errorf("seed %q: parse ICG frames %q: %w", pack.Name, vs.FramesFile, err)
				}
			} else {
				var frames []string
				if err := json.Unmarshal(framesData, &frames); err != nil {
					return fmt.Errorf("seed %q: parse frames %q: %w", pack.Name, vs.FramesFile, err)
				}
				icg, err = HTMLFramesToICG(frames, vs.Cols, vs.Rows)
				if err != nil {
					return fmt.Errorf("seed %q: convert frames %q: %w", pack.Name, vs.FramesFile, err)
				}
			}
			gz, firstFrame, err := CompressICG(icg)
			if err != nil {
				return fmt.Errorf("seed %q: compress frames %q: %w", pack.Name, vs.FramesFile, err)
			}
			paletteJSON, _ := json.Marshal(pack.Palette)
			if _, err := s.db.ExecContext(ctx,
				`INSERT OR REPLACE INTO size_variants(version_id, size, cols, rows, fps, palette, frames, first_frame) VALUES(?,?,?,?,?,?,?,?)`,
				verID, vs.Size, vs.Cols, vs.Rows, vs.FPS, string(paletteJSON), gz, firstFrame,
			); err != nil {
				return fmt.Errorf("seed %q/%q: %w", pack.Name, vs.Size, err)
			}
		}
	}
	return nil
}

// List returns metadata for all known animations without loading frame data.
func (s *SQLiteStore) List(ctx context.Context) ([]AnimationMeta, error) {
	rs, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.name, a.author, a.source, sv.size, sv.cols, sv.rows, sv.fps
		FROM animations a
		JOIN animation_versions av ON av.animation_id = a.id
		JOIN size_variants sv ON sv.version_id = av.id
		WHERE av.id = (
			SELECT id FROM animation_versions WHERE animation_id = a.id ORDER BY created_at DESC LIMIT 1
		)
		ORDER BY a.name, sv.size
	`)
	if err != nil {
		return nil, fmt.Errorf("list animations: %w", err)
	}
	defer rs.Close()

	var order []string
	byID := map[string]*AnimationMeta{}

	for rs.Next() {
		var id, name, author string
		var source sql.NullString
		var size sql.NullString
		var cols, rows, fps sql.NullInt64
		if err := rs.Scan(&id, &name, &author, &source, &size, &cols, &rows, &fps); err != nil {
			return nil, fmt.Errorf("scan animation row: %w", err)
		}
		if _, exists := byID[id]; !exists {
			byID[id] = &AnimationMeta{ID: id, Name: name, Author: author, Source: source.String}
			order = append(order, id)
		}
		if size.Valid {
			byID[id].Variants = append(byID[id].Variants, VariantMeta{
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
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

// ListSummaries returns animation metadata including first frames for gallery rendering.
func (s *SQLiteStore) ListSummaries(ctx context.Context) ([]AnimationSummary, error) {
	rs, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.name, a.author, COALESCE(a.pack_id,''), COALESCE(p.name,''), a.source, sv.size, sv.cols, sv.rows, sv.fps, sv.palette, sv.first_frame
		FROM animations a
		LEFT JOIN packs p ON p.id = a.pack_id
		JOIN animation_versions av ON av.animation_id = a.id
		JOIN size_variants sv ON sv.version_id = av.id
		WHERE av.id = (
			SELECT id FROM animation_versions WHERE animation_id = a.id ORDER BY created_at DESC LIMIT 1
		)
		ORDER BY a.pack_id NULLS FIRST, a.name, sv.size
	`)
	if err != nil {
		return nil, fmt.Errorf("list summaries: %w", err)
	}
	defer rs.Close()

	var order []string
	byID := map[string]*AnimationSummary{}

	for rs.Next() {
		var id, name, author, packID, packName string
		var source sql.NullString
		var size sql.NullString
		var cols, rows, fps sql.NullInt64
		var paletteJSON sql.NullString
		var firstFrame sql.NullString
		if err := rs.Scan(&id, &name, &author, &packID, &packName, &source, &size, &cols, &rows, &fps, &paletteJSON, &firstFrame); err != nil {
			return nil, fmt.Errorf("scan summary row: %w", err)
		}
		if _, exists := byID[id]; !exists {
			byID[id] = &AnimationSummary{ID: id, Name: name, Author: author, PackID: packID, PackName: packName, Source: source.String}
			order = append(order, id)
		}
		if size.Valid {
			var palette map[string]string
			if paletteJSON.Valid {
				json.Unmarshal([]byte(paletteJSON.String), &palette) //nolint:errcheck
			}
			byID[id].Variants = append(byID[id].Variants, VariantSummary{
				Size:       size.String,
				Cols:       int(cols.Int64),
				Rows:       int(rows.Int64),
				FPS:        int(fps.Int64),
				Palette:    palette,
				FirstFrame: firstFrame.String,
			})
		}
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("iterate summaries: %w", err)
	}

	result := make([]AnimationSummary, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

// ListSummariesPaged returns a paginated page of animation summaries.
func (s *SQLiteStore) ListSummariesPaged(ctx context.Context, query, sizeFilter string, page, pageSize int) (SummaryPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	queryPattern := "%" + query + "%"

	var total int
	countQuery := `
		SELECT COUNT(DISTINCT a.id)
		FROM animations a
		JOIN animation_versions av ON av.animation_id = a.id
		JOIN size_variants sv ON sv.version_id = av.id
		WHERE av.id = (
			SELECT id FROM animation_versions WHERE animation_id = a.id ORDER BY created_at DESC LIMIT 1
		)
		  AND (? = '' OR a.name LIKE ?)
		  AND (? = '' OR sv.size = ?)`
	if err := s.db.QueryRowContext(ctx, countQuery, query, queryPattern, sizeFilter, sizeFilter).Scan(&total); err != nil {
		return SummaryPage{}, fmt.Errorf("count paged summaries: %w", err)
	}

	nameRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT a.id, a.name, a.author, COALESCE(a.pack_id,''), COALESCE(p.name,''), a.source
		FROM animations a
		LEFT JOIN packs p ON p.id = a.pack_id
		JOIN animation_versions av ON av.animation_id = a.id
		JOIN size_variants sv ON sv.version_id = av.id
		WHERE av.id = (
			SELECT id FROM animation_versions WHERE animation_id = a.id ORDER BY created_at DESC LIMIT 1
		)
		  AND (? = '' OR a.name LIKE ?)
		  AND (? = '' OR sv.size = ?)
		ORDER BY a.pack_id NULLS FIRST, a.name
		LIMIT ? OFFSET ?`,
		query, queryPattern, sizeFilter, sizeFilter, pageSize, offset,
	)
	if err != nil {
		return SummaryPage{}, fmt.Errorf("list paged names: %w", err)
	}
	defer nameRows.Close()

	var order []string
	byID := map[string]*AnimationSummary{}
	for nameRows.Next() {
		var id, name, author, packID, packName string
		var source sql.NullString
		if err := nameRows.Scan(&id, &name, &author, &packID, &packName, &source); err != nil {
			return SummaryPage{}, fmt.Errorf("scan paged name row: %w", err)
		}
		byID[id] = &AnimationSummary{ID: id, Name: name, Author: author, PackID: packID, PackName: packName, Source: source.String}
		order = append(order, id)
	}
	if err := nameRows.Err(); err != nil {
		return SummaryPage{}, fmt.Errorf("iterate paged names: %w", err)
	}
	nameRows.Close()

	if len(order) == 0 {
		return SummaryPage{Total: total, Page: page, PageSize: pageSize}, nil
	}

	placeholders := make([]string, len(order))
	args := make([]any, 0, len(order)+2)
	for i, id := range order {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, sizeFilter, sizeFilter)

	dataQuery := `
		SELECT a.id, a.name, a.author, a.source, sv.size, sv.cols, sv.rows, sv.fps, sv.palette, sv.first_frame
		FROM animations a
		JOIN animation_versions av ON av.animation_id = a.id
		JOIN size_variants sv ON sv.version_id = av.id
		WHERE av.id = (
			SELECT id FROM animation_versions WHERE animation_id = a.id ORDER BY created_at DESC LIMIT 1
		)
		  AND a.id IN (` + strings.Join(placeholders, ",") + `)
		  AND (? = '' OR sv.size = ?)
		ORDER BY a.name, sv.size`
	rs, err := s.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return SummaryPage{}, fmt.Errorf("list paged data: %w", err)
	}
	defer rs.Close()

	for rs.Next() {
		var id, name, author string
		var source sql.NullString
		var size sql.NullString
		var cols, rows, fps sql.NullInt64
		var paletteJSON sql.NullString
		var firstFrame sql.NullString
		if err := rs.Scan(&id, &name, &author, &source, &size, &cols, &rows, &fps, &paletteJSON, &firstFrame); err != nil {
			return SummaryPage{}, fmt.Errorf("scan paged data row: %w", err)
		}
		anim, ok := byID[id]
		if !ok {
			continue
		}
		if size.Valid {
			var palette map[string]string
			if paletteJSON.Valid {
				json.Unmarshal([]byte(paletteJSON.String), &palette) //nolint:errcheck
			}
			anim.Variants = append(anim.Variants, VariantSummary{
				Size:       size.String,
				Cols:       int(cols.Int64),
				Rows:       int(rows.Int64),
				FPS:        int(fps.Int64),
				Palette:    palette,
				FirstFrame: firstFrame.String,
			})
		}
	}
	if err := rs.Err(); err != nil {
		return SummaryPage{}, fmt.Errorf("iterate paged data: %w", err)
	}

	result := make([]AnimationSummary, 0, len(order))
	for _, id := range order {
		if anim, ok := byID[id]; ok && len(anim.Variants) > 0 {
			result = append(result, *anim)
		}
	}

	return SummaryPage{
		Animations: result,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// ListDistinctSizes returns all distinct variant sizes in the store, sorted.
func (s *SQLiteStore) ListDistinctSizes(ctx context.Context) ([]string, error) {
	rs, err := s.db.QueryContext(ctx, `SELECT DISTINCT size FROM size_variants ORDER BY size`)
	if err != nil {
		return nil, fmt.Errorf("list distinct sizes: %w", err)
	}
	defer rs.Close()

	var sizes []string
	for rs.Next() {
		var size string
		if err := rs.Scan(&size); err != nil {
			return nil, fmt.Errorf("scan size row: %w", err)
		}
		sizes = append(sizes, size)
	}
	return sizes, rs.Err()
}

// Get returns all loaded variants (including frames) for the animation with the given UUID.
func (s *SQLiteStore) Get(ctx context.Context, id string) ([]AnimationVariant, error) {
	var animID, name string
	var source sql.NullString

	err := s.db.QueryRowContext(ctx, `SELECT id, name, source FROM animations WHERE id=?`, id).Scan(&animID, &name, &source)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get animation %q: %w", id, err)
	}

	var verID string
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM animation_versions WHERE animation_id=? ORDER BY created_at DESC LIMIT 1`, animID,
	).Scan(&verID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get version for %q: %w", id, err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT size, cols, rows, fps, palette, first_frame, frames
		FROM size_variants WHERE version_id=?
		ORDER BY size`, verID)
	if err != nil {
		return nil, fmt.Errorf("get size variants: %w", err)
	}
	defer rows.Close()

	var variants []AnimationVariant
	for rows.Next() {
		var size, paletteJSON, firstFrame string
		var cols, rws, fps int
		var framesGzip []byte
		if err := rows.Scan(&size, &cols, &rws, &fps, &paletteJSON, &firstFrame, &framesGzip); err != nil {
			return nil, fmt.Errorf("scan size variant: %w", err)
		}
		var palette map[string]string
		json.Unmarshal([]byte(paletteJSON), &palette) //nolint:errcheck
		variants = append(variants, AnimationVariant{
			ID:         animID,
			Name:       name,
			Source:     source.String,
			Size:       size,
			Cols:       cols,
			Rows:       rws,
			FPS:        fps,
			Palette:    palette,
			FirstFrame: firstFrame,
			FramesGzip: framesGzip,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate size variants: %w", err)
	}
	if len(variants) == 0 {
		return nil, ErrNotFound
	}
	return variants, nil
}

// GetVariant returns a specific size variant of the animation with the given UUID.
func (s *SQLiteStore) GetVariant(ctx context.Context, id, size string) (*AnimationVariant, error) {
	var animID, name string
	var source sql.NullString

	err := s.db.QueryRowContext(ctx, `SELECT id, name, source FROM animations WHERE id=?`, id).Scan(&animID, &name, &source)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get animation %q: %w", id, err)
	}

	var verID string
	err = s.db.QueryRowContext(ctx,
		`SELECT id FROM animation_versions WHERE animation_id=? ORDER BY created_at DESC LIMIT 1`, animID,
	).Scan(&verID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get version for %q: %w", id, err)
	}

	var paletteJSON, firstFrame string
	var cols, rows, fps int
	var framesGzip []byte
	err = s.db.QueryRowContext(ctx,
		`SELECT cols, rows, fps, palette, first_frame, frames FROM size_variants WHERE version_id=? AND size=?`,
		verID, size,
	).Scan(&cols, &rows, &fps, &paletteJSON, &firstFrame, &framesGzip)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get size variant %q/%q: %w", id, size, err)
	}

	var palette map[string]string
	json.Unmarshal([]byte(paletteJSON), &palette) //nolint:errcheck

	v := &AnimationVariant{
		ID:         animID,
		Name:       name,
		Source:     source.String,
		Size:       size,
		Cols:       cols,
		Rows:       rows,
		FPS:        fps,
		Palette:    palette,
		FirstFrame: firstFrame,
		FramesGzip: framesGzip,
	}
	return v, nil
}

// Put creates or replaces a variant and its frame data atomically. Returns the animation UUID.
func (s *SQLiteStore) Put(ctx context.Context, v AnimationVariant) (string, error) {
	if len(v.FramesGzip) == 0 {
		return "", fmt.Errorf("Put %q/%q: FramesGzip must not be empty; call CompressFrames first", v.Name, v.Size)
	}

	if err := SanitizePalette(v.Palette); err != nil {
		return "", fmt.Errorf("Put %q/%q: %w", v.Name, v.Size, err)
	}

	sanitized, err := sanitizeAndRecompressFrames(v)
	if err != nil {
		return "", fmt.Errorf("Put %q/%q: %w", v.Name, v.Size, err)
	}
	v = sanitized

	paletteJSON, err := json.Marshal(v.Palette)
	if err != nil {
		return "", fmt.Errorf("marshal palette: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	author := "user"
	if v.PackID != "" {
		// Use the pack's author for animations that belong to a pack.
		tx.QueryRowContext(ctx, `SELECT author FROM packs WHERE id=?`, v.PackID).Scan(&author) //nolint:errcheck
	}

	var animID string
	if v.ID != "" {
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM animations WHERE id=?`, v.ID,
		).Scan(&animID)
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("%w: animation %q", ErrNotFound, v.ID)
		}
	} else if v.PackID != "" {
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM animations WHERE pack_id=? AND name=?`, v.PackID, v.Name,
		).Scan(&animID)
	} else {
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM animations WHERE author=? AND name=? AND pack_id IS NULL`, author, v.Name,
		).Scan(&animID)
	}
	if err == sql.ErrNoRows {
		animID = newUUID()
		var src interface{} = nil
		if v.Source != "" {
			src = v.Source
		}
		var packID interface{} = nil
		if v.PackID != "" {
			packID = v.PackID
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO animations(id, author, pack_id, name, source, tags, created_at) VALUES(?,?,?,?,?,?,?)`,
			animID, author, packID, v.Name, src, "[]", now,
		); err != nil {
			return "", fmt.Errorf("insert animation: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("lookup animation: %w", err)
	} else {
		var packSrc sql.NullString
		tx.QueryRowContext(ctx, //nolint:errcheck
			`SELECT p.source FROM animations a LEFT JOIN packs p ON p.id = a.pack_id WHERE a.id=?`, animID,
		).Scan(&packSrc)
		if packSrc.Valid && packSrc.String != "" {
			return "", fmt.Errorf("%w: animation belongs to remote pack", ErrReadOnly)
		}
		if v.Source != "" {
			tx.ExecContext(ctx, `UPDATE animations SET source=? WHERE id=?`, v.Source, animID) //nolint:errcheck
		}
	}

	var verID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM animation_versions WHERE animation_id=? AND version='1.0.0'`, animID,
	).Scan(&verID)
	if err == sql.ErrNoRows {
		verID = newUUID()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO animation_versions(id, animation_id, version, created_at) VALUES(?,?,?,?)`,
			verID, animID, "1.0.0", now,
		); err != nil {
			return "", fmt.Errorf("insert version: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("lookup version: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO size_variants(version_id, size, cols, rows, fps, palette, frames, first_frame) VALUES(?,?,?,?,?,?,?,?)`,
		verID, v.Size, v.Cols, v.Rows, v.FPS, string(paletteJSON), v.FramesGzip, v.FirstFrame,
	); err != nil {
		return "", fmt.Errorf("upsert size variant: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	v.ID = animID
	s.broadcast(Event{Kind: EventPut, Variant: v, Name: animID, AnimationID: animID, VersionID: verID})
	return animID, nil
}

// Delete removes an animation and all its variants (cascade).
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	var animID string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM animations WHERE id=?`, id).Scan(&animID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup animation %q: %w", id, err)
	}

	if src := s.remotePackSource(ctx, animID); src != "" {
		return fmt.Errorf("%w: animation belongs to remote pack", ErrReadOnly)
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM animations WHERE id=?`, animID)
	if err != nil {
		return fmt.Errorf("delete animation %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.broadcast(Event{Kind: EventDelete, Name: animID, AnimationID: animID})
	return nil
}

// DeleteVariant removes a single size variant of an animation.
func (s *SQLiteStore) DeleteVariant(ctx context.Context, id, size string) error {
	var animID string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM animations WHERE id=?`, id).Scan(&animID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup animation: %w", err)
	}

	var verID string
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM animation_versions WHERE animation_id=? ORDER BY created_at DESC LIMIT 1`, animID,
	).Scan(&verID); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("lookup version: %w", err)
	}

	if src := s.remotePackSource(ctx, animID); src != "" {
		return fmt.Errorf("%w: animation belongs to remote pack", ErrReadOnly)
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM size_variants WHERE version_id=? AND size=?`, verID, size)
	if err != nil {
		return fmt.Errorf("delete size variant: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
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
				close(ch)
				break
			}
		}
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

// broadcast sends ev to all active Watch subscribers.
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

// --- Pack methods ---

func (s *SQLiteStore) ListPacks(ctx context.Context, author string) ([]Pack, error) {
	q := `SELECT id, author, name, description, license, source, tags, created_at FROM packs`
	args := []any{}
	if author != "" {
		q += ` WHERE author=?`
		args = append(args, author)
	}
	q += ` ORDER BY author, name`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list packs: %w", err)
	}
	defer rows.Close()

	var packs []Pack
	for rows.Next() {
		var p Pack
		var source sql.NullString
		var tagsJSON string
		if err := rows.Scan(&p.ID, &p.Author, &p.Name, &p.Description, &p.License, &source, &tagsJSON, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pack: %w", err)
		}
		p.Source = source.String
		json.Unmarshal([]byte(tagsJSON), &p.Tags) //nolint:errcheck
		if p.Tags == nil {
			p.Tags = []string{}
		}
		packs = append(packs, p)
	}
	return packs, rows.Err()
}

func (s *SQLiteStore) GetPack(ctx context.Context, id string) (*Pack, error) {
	var p Pack
	var source sql.NullString
	var tagsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, author, name, description, license, source, tags, created_at FROM packs WHERE id=?`, id,
	).Scan(&p.ID, &p.Author, &p.Name, &p.Description, &p.License, &source, &tagsJSON, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get pack %q: %w", id, err)
	}
	p.Source = source.String
	json.Unmarshal([]byte(tagsJSON), &p.Tags) //nolint:errcheck
	if p.Tags == nil {
		p.Tags = []string{}
	}
	return &p, nil
}

func (s *SQLiteStore) PutPack(ctx context.Context, p Pack) error {
	if p.ID == "" {
		p.ID = newUUID()
	}
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	tagsJSON, _ := json.Marshal(p.Tags)
	var src interface{} = nil
	if p.Source != "" {
		src = p.Source
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO packs(id, author, name, description, license, source, tags, created_at) VALUES(?,?,?,?,?,?,?,?)`,
		p.ID, p.Author, p.Name, p.Description, p.License, src, string(tagsJSON), p.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) DeletePack(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM packs WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete pack %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Animation v2 methods ---

func (s *SQLiteStore) ListAnimationsV2(ctx context.Context, filters AnimationFilters) ([]Animation, error) {
	q := `
		SELECT a.id, a.author, COALESCE(a.pack_id,''), COALESCE(p.name,''), a.name, a.tags, COALESCE(a.source,''), a.created_at
		FROM animations a
		LEFT JOIN packs p ON p.id = a.pack_id
		WHERE 1=1`
	args := []any{}
	if filters.Author != "" {
		q += ` AND a.author=?`
		args = append(args, filters.Author)
	}
	if filters.PackID != "" {
		q += ` AND a.pack_id=?`
		args = append(args, filters.PackID)
	}
	if filters.Query != "" {
		q += ` AND a.name LIKE ?`
		args = append(args, "%"+filters.Query+"%")
	}
	q += ` ORDER BY a.author, a.name`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list animations v2: %w", err)
	}
	defer rows.Close()

	var anims []Animation
	for rows.Next() {
		var a Animation
		var tagsJSON string
		if err := rows.Scan(&a.ID, &a.Author, &a.PackID, &a.PackName, &a.Name, &tagsJSON, &a.Source, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan animation v2: %w", err)
		}
		json.Unmarshal([]byte(tagsJSON), &a.Tags) //nolint:errcheck
		if a.Tags == nil {
			a.Tags = []string{}
		}
		anims = append(anims, a)
	}
	return anims, rows.Err()
}

func (s *SQLiteStore) GetAnimationByID(ctx context.Context, id string) (*Animation, error) {
	var a Animation
	var tagsJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.author, COALESCE(a.pack_id,''), COALESCE(p.name,''), a.name, a.tags, COALESCE(a.source,''), a.created_at
		FROM animations a LEFT JOIN packs p ON p.id = a.pack_id
		WHERE a.id=?`, id,
	).Scan(&a.ID, &a.Author, &a.PackID, &a.PackName, &a.Name, &tagsJSON, &a.Source, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get animation by id %q: %w", id, err)
	}
	json.Unmarshal([]byte(tagsJSON), &a.Tags) //nolint:errcheck
	if a.Tags == nil {
		a.Tags = []string{}
	}
	return &a, nil
}

func (s *SQLiteStore) GetAnimationByName(ctx context.Context, author, name string) (*Animation, error) {
	var a Animation
	var tagsJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.author, COALESCE(a.pack_id,''), COALESCE(p.name,''), a.name, a.tags, COALESCE(a.source,''), a.created_at
		FROM animations a LEFT JOIN packs p ON p.id = a.pack_id
		WHERE a.author=? AND a.name=? AND a.pack_id IS NULL`, author, name,
	).Scan(&a.ID, &a.Author, &a.PackID, &a.PackName, &a.Name, &tagsJSON, &a.Source, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get animation by name %q/%q: %w", author, name, err)
	}
	json.Unmarshal([]byte(tagsJSON), &a.Tags) //nolint:errcheck
	if a.Tags == nil {
		a.Tags = []string{}
	}
	return &a, nil
}

func (s *SQLiteStore) GetAnimationInPack(ctx context.Context, packID, name string) (*Animation, error) {
	var a Animation
	var tagsJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.author, COALESCE(a.pack_id,''), COALESCE(p.name,''), a.name, a.tags, COALESCE(a.source,''), a.created_at
		FROM animations a LEFT JOIN packs p ON p.id = a.pack_id
		WHERE a.pack_id=? AND a.name=?`, packID, name,
	).Scan(&a.ID, &a.Author, &a.PackID, &a.PackName, &a.Name, &tagsJSON, &a.Source, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get animation in pack %q/%q: %w", packID, name, err)
	}
	json.Unmarshal([]byte(tagsJSON), &a.Tags) //nolint:errcheck
	if a.Tags == nil {
		a.Tags = []string{}
	}
	return &a, nil
}

// remotePackSource returns the source URL of the pack an animation belongs to,
// or "" if the animation is ungrouped or the pack is local.
func (s *SQLiteStore) remotePackSource(ctx context.Context, animID string) string {
	var src sql.NullString
	s.db.QueryRowContext(ctx, //nolint:errcheck
		`SELECT p.source FROM animations a LEFT JOIN packs p ON p.id = a.pack_id WHERE a.id=?`, animID,
	).Scan(&src)
	return src.String
}

func (s *SQLiteStore) PutAnimationV2(ctx context.Context, a Animation) error {
	if a.ID == "" {
		a.ID = newUUID()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}
	tagsJSON, _ := json.Marshal(a.Tags)
	var src interface{} = nil
	if a.Source != "" {
		src = a.Source
	}
	var packID interface{} = nil
	if a.PackID != "" {
		packID = a.PackID
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO animations(id, author, pack_id, name, tags, source, created_at) VALUES(?,?,?,?,?,?,?)`,
		a.ID, a.Author, packID, a.Name, string(tagsJSON), src, a.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) DeleteAnimationByID(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM animations WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete animation %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Version methods ---

func (s *SQLiteStore) ListVersions(ctx context.Context, animationID string) ([]VersionMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, animation_id, version, created_at FROM animation_versions WHERE animation_id=? ORDER BY created_at`,
		animationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []VersionMeta
	for rows.Next() {
		var v VersionMeta
		if err := rows.Scan(&v.ID, &v.AnimationID, &v.Version, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (s *SQLiteStore) GetLatestVersion(ctx context.Context, animationID string) (*VersionMeta, error) {
	var v VersionMeta
	err := s.db.QueryRowContext(ctx,
		`SELECT id, animation_id, version, created_at FROM animation_versions WHERE animation_id=? ORDER BY created_at DESC LIMIT 1`,
		animationID,
	).Scan(&v.ID, &v.AnimationID, &v.Version, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest version: %w", err)
	}
	return &v, nil
}

func (s *SQLiteStore) PutVersion(ctx context.Context, animationID string, version string) (string, error) {
	id := newUUID()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO animation_versions(id, animation_id, version, created_at) VALUES(?,?,?,?)`,
		id, animationID, version, now,
	)
	if err != nil {
		return "", fmt.Errorf("put version: %w", err)
	}
	var actualID string
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM animation_versions WHERE animation_id=? AND version=?`, animationID, version,
	).Scan(&actualID); err == nil {
		return actualID, nil
	}
	return id, nil
}

func (s *SQLiteStore) DeleteVersion(ctx context.Context, versionID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM animation_versions WHERE id=?`, versionID)
	if err != nil {
		return fmt.Errorf("delete version %q: %w", versionID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Size variant methods ---

func (s *SQLiteStore) GetSizeVariantByVersionID(ctx context.Context, versionID, size string) (*SizeVariantData, error) {
	var sv SizeVariantData
	var paletteJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT size, cols, rows, fps, palette, first_frame, frames FROM size_variants WHERE version_id=? AND size=?`,
		versionID, size,
	).Scan(&sv.Size, &sv.Cols, &sv.Rows, &sv.FPS, &paletteJSON, &sv.FirstFrame, &sv.FramesGzip)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get size variant: %w", err)
	}
	json.Unmarshal([]byte(paletteJSON), &sv.Palette) //nolint:errcheck
	return &sv, nil
}

func (s *SQLiteStore) PutSizeVariant(ctx context.Context, versionID string, sv SizeVariantData) error {
	paletteJSON, _ := json.Marshal(sv.Palette)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO size_variants(version_id, size, cols, rows, fps, palette, frames, first_frame) VALUES(?,?,?,?,?,?,?,?)`,
		versionID, sv.Size, sv.Cols, sv.Rows, sv.FPS, string(paletteJSON), sv.FramesGzip, sv.FirstFrame,
	)
	return err
}

func (s *SQLiteStore) ListSizeVariants(ctx context.Context, versionID string) ([]SizeVariantData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT size, cols, rows, fps, palette, first_frame FROM size_variants WHERE version_id=? ORDER BY size`,
		versionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list size variants: %w", err)
	}
	defer rows.Close()

	var svs []SizeVariantData
	for rows.Next() {
		var sv SizeVariantData
		var paletteJSON string
		if err := rows.Scan(&sv.Size, &sv.Cols, &sv.Rows, &sv.FPS, &paletteJSON, &sv.FirstFrame); err != nil {
			return nil, fmt.Errorf("scan size variant: %w", err)
		}
		json.Unmarshal([]byte(paletteJSON), &sv.Palette) //nolint:errcheck
		svs = append(svs, sv)
	}
	return svs, rows.Err()
}

// --- Query methods ---

func (s *SQLiteStore) ListAuthors(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT author FROM animations ORDER BY author`)
	if err != nil {
		return nil, fmt.Errorf("list authors: %w", err)
	}
	defer rows.Close()

	var authors []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, fmt.Errorf("scan author: %w", err)
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}

func (s *SQLiteStore) ListDistinctTagsV2(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tags FROM animations WHERE tags != '[]' AND tags != ''`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var tagsJSON string
		if err := rows.Scan(&tagsJSON); err != nil {
			continue
		}
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			continue
		}
		for _, t := range tags {
			seen[t] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(seen))
	for t := range seen {
		result = append(result, t)
	}
	return result, nil
}
