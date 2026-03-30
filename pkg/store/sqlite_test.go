package store

import (
	"context"
	"errors"
	"testing"
)

// makeICGData builds a minimal ICG data structure for testing.
func makeICGData(t *testing.T, numFrames int, cols, rows int) *ICGData {
	t.Helper()
	data := &ICGData{
		ClassTable: []string{"", "fg"},
		Frames:     make([]ICGFrame, numFrames),
	}
	for i := range data.Frames {
		line := make([]byte, cols)
		for j := range line {
			line[j] = ' '
		}
		lineStr := string(line)
		lines := make([]string, rows)
		for j := range lines {
			lines[j] = lineStr
		}
		chars := ""
		for j, l := range lines {
			if j > 0 {
				chars += "\n"
			}
			chars += l
		}
		colors := make([]byte, cols*rows)
		data.Frames[i] = ICGFrame{Chars: chars, Colors: colors}
	}
	return data
}

func makeVariant(t *testing.T, frames []string) AnimationVariant {
	t.Helper()
	// Convert HTML frames to ICG for backwards compatibility with existing tests.
	icg, err := HTMLFramesToICG(frames, 80, 40)
	if err != nil {
		t.Fatalf("HTMLFramesToICG: %v", err)
	}
	gz, first, err := CompressICG(icg)
	if err != nil {
		t.Fatalf("CompressICG: %v", err)
	}
	return AnimationVariant{
		Name:       "test",
		Size:       "1x1",
		Cols:       80,
		Rows:       40,
		FPS:        24,
		FirstFrame: first,
		FramesGzip: gz,
	}
}

// TestFramesStoredCompressed verifies that Put stores a gzip blob and first_frame
// in the DB, and Get returns them intact without decompression.
func TestFramesStoredCompressed(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	frames := []string{"<span>frame0</span>", "<span>frame1</span>"}
	v := makeVariant(t, frames)

	animID, err := st.Put(ctx, v)
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	// Blob in DB must start with gzip magic bytes 0x1f 0x8b.
	var blob []byte
	if err := st.db.QueryRowContext(ctx, `
		SELECT sv.frames FROM size_variants sv
		JOIN animation_versions av ON av.id = sv.version_id
		JOIN animations a ON a.id = av.animation_id
		WHERE a.name = 'test' AND sv.size = '1x1'`,
	).Scan(&blob); err != nil {
		t.Fatalf("query blob: %v", err)
	}
	if len(blob) < 2 || blob[0] != 0x1f || blob[1] != 0x8b {
		t.Fatalf("blob is not gzip: first bytes %x", blob[:min(len(blob), 4)])
	}

	// first_frame column must be non-empty (generated from ICG frame[0]).
	var dbFirstFrame string
	if err := st.db.QueryRowContext(ctx, `
		SELECT sv.first_frame FROM size_variants sv
		JOIN animation_versions av ON av.id = sv.version_id
		JOIN animations a ON a.id = av.animation_id
		WHERE a.name = 'test' AND sv.size = '1x1'`,
	).Scan(&dbFirstFrame); err != nil {
		t.Fatalf("query first_frame: %v", err)
	}
	if dbFirstFrame == "" {
		t.Fatal("first_frame: expected non-empty HTML string")
	}

	// Get returns FirstFrame and FramesGzip intact — no decompression.
	variants, err := st.Get(ctx, animID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	got := variants[0]
	if got.FirstFrame == "" {
		t.Error("FirstFrame: expected non-empty")
	}
	if len(got.FramesGzip) < 2 || got.FramesGzip[0] != 0x1f || got.FramesGzip[1] != 0x8b {
		t.Errorf("FramesGzip is not gzip: first bytes %x", got.FramesGzip[:min(len(got.FramesGzip), 4)])
	}

	// FramesGzip must decompress to valid ICG data.
	icg, err := DecompressICG(got.FramesGzip)
	if err != nil {
		t.Fatalf("DecompressICG: %v", err)
	}
	if len(icg.Frames) != len(frames) {
		t.Fatalf("ICG frames count: want %d, got %d", len(frames), len(icg.Frames))
	}
}

// TestEventCarriesFirstFrameAndGzip verifies the Event broadcast from Put
// carries FirstFrame and FramesGzip on the variant.
func TestEventCarriesFirstFrameAndGzip(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := st.Watch(ctx)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	frames := []string{"<span>frame0</span>"}
	v := makeVariant(t, frames)

	if _, err := st.Put(ctx, v); err != nil {
		t.Fatalf("put: %v", err)
	}

	ev := <-ch
	if ev.Variant.FirstFrame == "" {
		t.Error("event FirstFrame: expected non-empty")
	}
	if len(ev.Variant.FramesGzip) < 2 || ev.Variant.FramesGzip[0] != 0x1f || ev.Variant.FramesGzip[1] != 0x8b {
		t.Errorf("event FramesGzip is not gzip: first bytes %x", ev.Variant.FramesGzip[:min(len(ev.Variant.FramesGzip), 4)])
	}
}

// TestListSummaries verifies ListSummaries returns summaries with first frames and palettes.
func TestListSummaries(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// Empty DB: returns empty slice.
	summaries, err := st.ListSummaries(ctx)
	if err != nil {
		t.Fatalf("ListSummaries empty: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries, got %d", len(summaries))
	}

	// Put two variants for the same animation.
	icg1 := makeICGData(t, 2, 80, 24)
	gz, first, err := CompressICG(icg1)
	if err != nil {
		t.Fatalf("CompressICG: %v", err)
	}
	if _, err := st.Put(ctx, AnimationVariant{
		Name:       "demo",
		Size:       "1x1",
		Cols:       80,
		Rows:       24,
		FPS:        12,
		Palette:    map[string]string{"red": "#ff0000"},
		FirstFrame: first,
		FramesGzip: gz,
	}); err != nil {
		t.Fatalf("put 1x1: %v", err)
	}
	icg2 := makeICGData(t, 1, 160, 24)
	gz2, first2, err := CompressICG(icg2)
	if err != nil {
		t.Fatalf("CompressICG 2x1: %v", err)
	}
	if _, err := st.Put(ctx, AnimationVariant{
		Name:       "demo",
		Size:       "2x1",
		Cols:       160,
		Rows:       24,
		FPS:        24,
		FirstFrame: first2,
		FramesGzip: gz2,
	}); err != nil {
		t.Fatalf("put 2x1: %v", err)
	}

	summaries, err = st.ListSummaries(ctx)
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 animation, got %d", len(summaries))
	}
	anim := summaries[0]
	if anim.Name != "demo" {
		t.Errorf("Name: want %q, got %q", "demo", anim.Name)
	}
	if len(anim.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(anim.Variants))
	}
	v1 := anim.Variants[0]
	if v1.Size != "1x1" {
		t.Errorf("v1.Size: want %q, got %q", "1x1", v1.Size)
	}
	if v1.FirstFrame == "" {
		t.Error("v1.FirstFrame: expected non-empty")
	}
	if v1.Palette["red"] != "#ff0000" {
		t.Errorf("v1.Palette[red]: want #ff0000, got %q", v1.Palette["red"])
	}
	v2 := anim.Variants[1]
	if v2.Size != "2x1" {
		t.Errorf("v2.Size: want %q, got %q", "2x1", v2.Size)
	}
	if v2.FirstFrame == "" {
		t.Error("v2.FirstFrame: expected non-empty")
	}
}

// TestPutRejectsEmptyFramesGzip verifies that Put returns an error when
// FramesGzip is not set, catching callers that forget CompressFrames.
func TestPutRejectsEmptyFramesGzip(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	_, err = st.Put(context.Background(), AnimationVariant{
		Name: "test",
		Size: "1x1",
	})
	if err == nil {
		t.Fatal("expected error for empty FramesGzip, got nil")
	}
}

// TestPutRejectsInvalidPalette verifies Put returns ErrInvalidInput for bad palette entries.
func TestPutRejectsInvalidPalette(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	icg := makeICGData(t, 1, 80, 24)
	gz, first, err := CompressICG(icg)
	if err != nil {
		t.Fatalf("CompressICG: %v", err)
	}

	cases := []struct {
		name    string
		palette map[string]string
	}{
		{"bad class name", map[string]string{"1bad": "#fff"}},
		{"class with dot", map[string]string{"has.dot": "#fff"}},
		{"bad color url", map[string]string{"ring": "url(evil)"}},
		{"bad color expression", map[string]string{"ring": "expression(x)"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := AnimationVariant{
				Name:       "test",
				Size:       "1x1",
				Cols:       80, Rows: 24, FPS: 12,
				Palette:    tc.palette,
				FirstFrame: first,
				FramesGzip: gz,
			}
			_, putErr := st.Put(ctx, v)
			if putErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(putErr, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got: %v", putErr)
			}
		})
	}
}

// TestListSummariesPaged verifies pagination, filtering, and count behaviour.
func TestListSummariesPaged(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// Seed 5 animations: alpha, beta, gamma, delta, epsilon.
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	for _, name := range names {
		icg := makeICGData(t, 1, 80, 24)
		gz, first, err := CompressICG(icg)
		if err != nil {
			t.Fatalf("CompressICG %s: %v", name, err)
		}
		if _, err := st.Put(ctx, AnimationVariant{
			Name: name, Size: "1x1",
			Cols: 80, Rows: 24, FPS: 12,
			FirstFrame: first, FramesGzip: gz,
		}); err != nil {
			t.Fatalf("put %s/1x1: %v", name, err)
		}
		// Also add a 2x1 variant for "alpha" only.
		if name == "alpha" {
			icg2 := makeICGData(t, 1, 160, 24)
			gz2, first2, err := CompressICG(icg2)
			if err != nil {
				t.Fatalf("CompressICG 2x1: %v", err)
			}
			if _, err := st.Put(ctx, AnimationVariant{
				Name: name, Size: "2x1",
				Cols: 160, Rows: 24, FPS: 12,
				FirstFrame: first2, FramesGzip: gz2,
			}); err != nil {
				t.Fatalf("put %s/2x1: %v", name, err)
			}
		}
	}

	// Page 1 of 2 (pageSize=3): should return 3 animations.
	pg, err := st.ListSummariesPaged(ctx, "", "", 1, 3)
	if err != nil {
		t.Fatalf("ListSummariesPaged p1: %v", err)
	}
	if pg.Total != 5 {
		t.Errorf("Total: want 5, got %d", pg.Total)
	}
	if len(pg.Animations) != 3 {
		t.Errorf("page 1 count: want 3, got %d", len(pg.Animations))
	}
	if pg.Page != 1 {
		t.Errorf("Page: want 1, got %d", pg.Page)
	}

	// Page 2 should return 2 animations.
	pg2, err := st.ListSummariesPaged(ctx, "", "", 2, 3)
	if err != nil {
		t.Fatalf("ListSummariesPaged p2: %v", err)
	}
	if len(pg2.Animations) != 2 {
		t.Errorf("page 2 count: want 2, got %d", len(pg2.Animations))
	}

	// Query filter: "a" matches alpha, beta, gamma, delta (4 of 5 names).
	pgQ, err := st.ListSummariesPaged(ctx, "a", "", 1, 10)
	if err != nil {
		t.Fatalf("ListSummariesPaged query: %v", err)
	}
	if pgQ.Total != 4 {
		t.Errorf("query 'a' total: want 4 (alpha, beta, gamma, delta), got %d", pgQ.Total)
	}

	// Size filter: "2x1" matches only alpha.
	pgS, err := st.ListSummariesPaged(ctx, "", "2x1", 1, 10)
	if err != nil {
		t.Fatalf("ListSummariesPaged size: %v", err)
	}
	if pgS.Total != 1 {
		t.Errorf("size '2x1' total: want 1, got %d", pgS.Total)
	}
	if len(pgS.Animations) != 1 || pgS.Animations[0].Name != "alpha" {
		t.Errorf("size '2x1': expected alpha, got %v", pgS.Animations)
	}
	// Only the 2x1 variant should be returned for alpha.
	if len(pgS.Animations[0].Variants) != 1 || pgS.Animations[0].Variants[0].Size != "2x1" {
		t.Errorf("size '2x1': expected single 2x1 variant for alpha, got %v", pgS.Animations[0].Variants)
	}

	// Empty DB page returns zero results.
	pgEmpty, err := st.ListSummariesPaged(ctx, "zzz-no-match", "", 1, 10)
	if err != nil {
		t.Fatalf("ListSummariesPaged no-match: %v", err)
	}
	if pgEmpty.Total != 0 || len(pgEmpty.Animations) != 0 {
		t.Errorf("no-match: expected 0, got total=%d anim=%d", pgEmpty.Total, len(pgEmpty.Animations))
	}
}

// TestListDistinctSizes verifies all distinct sizes are returned.
func TestListDistinctSizes(t *testing.T) {
	st, err := OpenSQLite(":memory:", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// Empty store.
	sizes, err := st.ListDistinctSizes(ctx)
	if err != nil {
		t.Fatalf("ListDistinctSizes empty: %v", err)
	}
	if len(sizes) != 0 {
		t.Errorf("expected 0 sizes, got %v", sizes)
	}

	// Add variants with two sizes.
	for _, size := range []string{"1x1", "2x1", "1x1"} {
		name := "anim-" + size
		icg := makeICGData(t, 1, 80, 24)
		gz, first, err := CompressICG(icg)
		if err != nil {
			t.Fatalf("CompressICG: %v", err)
		}
		if _, err := st.Put(ctx, AnimationVariant{
			Name: name, Size: size,
			Cols: 80, Rows: 24, FPS: 12,
			FirstFrame: first, FramesGzip: gz,
		}); err != nil {
			t.Fatalf("put %s/%s: %v", name, size, err)
		}
	}

	sizes, err = st.ListDistinctSizes(ctx)
	if err != nil {
		t.Fatalf("ListDistinctSizes: %v", err)
	}
	if len(sizes) != 2 {
		t.Errorf("expected 2 distinct sizes, got %v", sizes)
	}
}
