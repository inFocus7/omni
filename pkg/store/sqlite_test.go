package store

import (
	"context"
	"encoding/json"
	"testing"
)

func makeVariant(t *testing.T, frames []string) AnimationVariant {
	t.Helper()
	gz, first, err := CompressFrames(frames)
	if err != nil {
		t.Fatalf("CompressFrames: %v", err)
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

	if err := st.Put(ctx, v); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Blob in DB must start with gzip magic bytes 0x1f 0x8b.
	var blob []byte
	if err := st.db.QueryRowContext(ctx,
		"SELECT frames FROM animation_frames WHERE name = 'test' AND size = '1x1'",
	).Scan(&blob); err != nil {
		t.Fatalf("query blob: %v", err)
	}
	if len(blob) < 2 || blob[0] != 0x1f || blob[1] != 0x8b {
		t.Fatalf("blob is not gzip: first bytes %x", blob[:min(len(blob), 4)])
	}

	// first_frame column must equal frames[0].
	var dbFirstFrame string
	if err := st.db.QueryRowContext(ctx,
		"SELECT first_frame FROM animation_frames WHERE name = 'test' AND size = '1x1'",
	).Scan(&dbFirstFrame); err != nil {
		t.Fatalf("query first_frame: %v", err)
	}
	if dbFirstFrame != frames[0] {
		t.Fatalf("first_frame: want %q, got %q", frames[0], dbFirstFrame)
	}

	// Get returns FirstFrame and FramesGzip intact — no decompression.
	variants, err := st.Get(ctx, "test")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	got := variants[0]
	if got.FirstFrame != frames[0] {
		t.Errorf("FirstFrame: want %q, got %q", frames[0], got.FirstFrame)
	}
	if len(got.FramesGzip) < 2 || got.FramesGzip[0] != 0x1f || got.FramesGzip[1] != 0x8b {
		t.Errorf("FramesGzip is not gzip: first bytes %x", got.FramesGzip[:min(len(got.FramesGzip), 4)])
	}

	// FramesGzip must decompress to the original frames.
	plain, err := GzipDecompress(got.FramesGzip)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	var roundtrip []string
	if err := json.Unmarshal(plain, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for i, f := range frames {
		if roundtrip[i] != f {
			t.Errorf("frame[%d]: want %q, got %q", i, f, roundtrip[i])
		}
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

	if err := st.Put(ctx, v); err != nil {
		t.Fatalf("put: %v", err)
	}

	ev := <-ch
	if ev.Variant.FirstFrame != frames[0] {
		t.Errorf("event FirstFrame: want %q, got %q", frames[0], ev.Variant.FirstFrame)
	}
	if len(ev.Variant.FramesGzip) < 2 || ev.Variant.FramesGzip[0] != 0x1f || ev.Variant.FramesGzip[1] != 0x8b {
		t.Errorf("event FramesGzip is not gzip: first bytes %x", ev.Variant.FramesGzip[:min(len(ev.Variant.FramesGzip), 4)])
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

	err = st.Put(context.Background(), AnimationVariant{
		Name: "test",
		Size: "1x1",
	})
	if err == nil {
		t.Fatal("expected error for empty FramesGzip, got nil")
	}
}
