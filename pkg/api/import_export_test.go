package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/infocus7/omni/pkg/api"
	"github.com/infocus7/omni/pkg/store"
	"github.com/infocus7/omni/pkg/widgets"
)

// ── Test server ───────────────────────────────────────────────────────────────

type testEnv struct {
	srv   *httptest.Server
	store *store.SQLiteStore
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	st, err := store.OpenSQLite(":memory:", nil)
	require.NoError(t, err, "open in-memory sqlite")
	t.Cleanup(func() { st.Close() })

	reg := widgets.NewRegistry()
	cache := &sync.Map{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	ascii := api.NewAsciiAPI(st, reg, cache)
	ascii.RegisterRoutes(r)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &testEnv{srv: srv, store: st}
}

// get issues a GET request and returns the response.
func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	res, err := http.Get(e.srv.URL + path)
	require.NoError(t, err)
	return res
}

// postJSON issues a POST with a JSON body.
func (e *testEnv) postJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	res, err := http.Post(e.srv.URL+path, "application/json", bytes.NewReader(data))
	require.NoError(t, err)
	return res
}

// postMultipart issues a POST with a multipart/form-data body where each
// entry in files is a form field named "files" with the relative path as
// the filename (mirroring what the browser sends via webkitdirectory).
func (e *testEnv) postMultipart(t *testing.T, path string, files map[string][]byte) *http.Response {
	t.Helper()
	body, ct := buildMultipart(t, files)
	res, err := http.Post(e.srv.URL+path, ct, body)
	require.NoError(t, err)
	return res
}

// ── Multipart builder ─────────────────────────────────────────────────────────

func buildMultipart(t *testing.T, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for relPath, content := range files {
		// Use the relative path as the FIELD NAME (not the filename), because Go's
		// multipart parser strips directory components from filenames for security.
		// The handler reads field names to recover the full relative path.
		part, err := mw.CreateFormFile(relPath, path.Base(relPath))
		require.NoError(t, err, "create form file %q", relPath)
		_, err = part.Write(content)
		require.NoError(t, err, "write form file %q", relPath)
	}
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// ── Fixture data ──────────────────────────────────────────────────────────────

// singleAnimFiles returns a minimal valid single-animation file set with the
// given animation name, rooted at "<name>/".
func singleAnimFiles(name string) map[string][]byte {
	meta := map[string]any{
		"name":    name,
		"palette": map[string]string{"fg": "#cccccc"},
		"variants": []map[string]any{{
			"size":        "1x1",
			"cols":        40,
			"rows":        12,
			"fps":         10,
			"frames_file": "frames-1x1.json",
		}},
	}
	metaJSON, _ := json.Marshal(meta)
	frames, _ := json.Marshal([]string{
		`<span class="fg">frame0</span>`,
		`<span class="fg">frame1</span>`,
	})
	return map[string][]byte{
		name + "/meta.json":        metaJSON,
		name + "/frames-1x1.json": frames,
	}
}

// multiVariantAnimFiles returns a single animation with two size variants.
func multiVariantAnimFiles(name string) map[string][]byte {
	meta := map[string]any{
		"name": name,
		"variants": []map[string]any{
			{"size": "1x1", "cols": 40, "rows": 12, "fps": 10, "frames_file": "frames-1x1.json"},
			{"size": "2x1", "cols": 80, "rows": 12, "fps": 10, "frames_file": "frames-2x1.json"},
		},
	}
	metaJSON, _ := json.Marshal(meta)
	frames, _ := json.Marshal([]string{`<span>f0</span>`})
	return map[string][]byte{
		name + "/meta.json":        metaJSON,
		name + "/frames-1x1.json": frames,
		name + "/frames-2x1.json": frames,
	}
}

// packFiles returns a pack with multiple animations, rooted at "<packname>/".
func packFiles(packName string, animNames ...string) map[string][]byte {
	pack := map[string]any{
		"name":       packName,
		"version":    "1.0.0",
		"author":     "test",
		"animations": animNames,
	}
	packJSON, _ := json.Marshal(pack)
	files := map[string][]byte{
		packName + "/pack.json": packJSON,
	}
	for _, name := range animNames {
		for relPath, content := range singleAnimFiles(name) {
			// re-root under packName/
			files[packName+"/"+relPath] = content
		}
	}
	return files
}

// ── Zip helpers ───────────────────────────────────────────────────────────────

func readZip(t *testing.T, body io.Reader) []*zip.File {
	t.Helper()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err, "parse zip response")
	return r.File
}

func zipPaths(files []*zip.File) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Name
	}
	return paths
}

func zipFileContent(t *testing.T, files []*zip.File, name string) []byte {
	t.Helper()
	for _, f := range files {
		if f.Name == name {
			rc, err := f.Open()
			require.NoError(t, err)
			defer rc.Close()
			data, err := io.ReadAll(rc)
			require.NoError(t, err)
			return data
		}
	}
	t.Fatalf("zip entry %q not found in %v", name, zipPaths(files))
	return nil
}

// ── JSON decode helper ────────────────────────────────────────────────────────

func decodeJSON(t *testing.T, r io.Reader, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(r).Decode(dst))
}

// ── Import tests ──────────────────────────────────────────────────────────────

func TestImport_SingleAnimation_HappyPath(t *testing.T) {
	e := newTestEnv(t)

	res := e.postMultipart(t, "/api/ascii/import", singleAnimFiles("logo"))
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	require.Len(t, result.Imported, 1)
	assert.Equal(t, "logo", result.Imported[0].Name)
	assert.Equal(t, []string{"1x1"}, result.Imported[0].Sizes)
	assert.Empty(t, result.Skipped)
	assert.Empty(t, result.Conflicts)

	// Verify animation is actually in the store.
	variants, err := e.store.Get(context.Background(), "logo")
	require.NoError(t, err)
	assert.Len(t, variants, 1)
	assert.Equal(t, "1x1", variants[0].Size)
	assert.Empty(t, variants[0].Source, "imported animation must be local (source empty)")
}

func TestImport_SingleAnimation_MultipleVariants(t *testing.T) {
	e := newTestEnv(t)

	res := e.postMultipart(t, "/api/ascii/import", multiVariantAnimFiles("spinner"))
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	require.Len(t, result.Imported, 1)
	assert.Equal(t, "spinner", result.Imported[0].Name)
	assert.ElementsMatch(t, []string{"1x1", "2x1"}, result.Imported[0].Sizes)

	variants, err := e.store.Get(context.Background(), "spinner")
	require.NoError(t, err)
	assert.Len(t, variants, 2)
}

func TestImport_SingleAnimation_NameCollision_Returns409(t *testing.T) {
	e := newTestEnv(t)

	// First import succeeds.
	res := e.postMultipart(t, "/api/ascii/import", singleAnimFiles("logo"))
	res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	// Second import of the same name without overwrite → 409.
	res2 := e.postMultipart(t, "/api/ascii/import", singleAnimFiles("logo"))
	defer res2.Body.Close()

	require.Equal(t, http.StatusConflict, res2.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res2.Body, &result)

	assert.Contains(t, result.Conflicts, "logo")
	assert.Empty(t, result.Imported)
}

func TestImport_SingleAnimation_NameCollision_OverwriteSucceeds(t *testing.T) {
	e := newTestEnv(t)

	// Seed an existing animation.
	res := e.postMultipart(t, "/api/ascii/import", singleAnimFiles("logo"))
	res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	// Re-import with ?overwrite=true → 200, animation replaced.
	res2 := e.postMultipart(t, "/api/ascii/import?overwrite=true", singleAnimFiles("logo"))
	defer res2.Body.Close()

	require.Equal(t, http.StatusOK, res2.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res2.Body, &result)

	assert.Len(t, result.Imported, 1)
	assert.Empty(t, result.Conflicts)
}

func TestImport_Pack_HappyPath(t *testing.T) {
	e := newTestEnv(t)

	files := packFiles("my-pack", "anim1", "anim2", "anim3")
	res := e.postMultipart(t, "/api/ascii/import", files)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	importedNames := make([]string, len(result.Imported))
	for i, a := range result.Imported {
		importedNames[i] = a.Name
	}
	assert.ElementsMatch(t, []string{"anim1", "anim2", "anim3"}, importedNames)
	assert.Empty(t, result.Skipped)

	// All three must be in the store.
	for _, name := range []string{"anim1", "anim2", "anim3"} {
		_, err := e.store.Get(context.Background(), name)
		assert.NoError(t, err, "animation %q should exist after pack import", name)
	}
}

func TestImport_Pack_PartialFailure_MissingAnimation(t *testing.T) {
	e := newTestEnv(t)

	// Pack declares "anim1" and "anim2" but files only include "anim1".
	packJSON, _ := json.Marshal(map[string]any{
		"name":       "partial-pack",
		"version":    "1.0.0",
		"animations": []string{"anim1", "anim2"},
	})
	files := map[string][]byte{
		"partial-pack/pack.json": packJSON,
	}
	for k, v := range singleAnimFiles("anim1") {
		files["partial-pack/"+k] = v
	}

	res := e.postMultipart(t, "/api/ascii/import", files)
	defer res.Body.Close()

	// Partial success is still 200 — result body carries the detail.
	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	assert.Len(t, result.Imported, 1, "anim1 should be imported")
	assert.Len(t, result.Skipped, 1, "anim2 should be skipped")
	assert.Equal(t, "anim2", result.Skipped[0].Name)
	assert.NotEmpty(t, result.Skipped[0].Reason)
}

func TestImport_InvalidMetaJSON_ReturnsSkipped(t *testing.T) {
	e := newTestEnv(t)

	files := map[string][]byte{
		"bad/meta.json":        []byte(`not valid json`),
		"bad/frames-1x1.json": []byte(`["frame"]`),
	}

	res := e.postMultipart(t, "/api/ascii/import", files)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	assert.Empty(t, result.Imported)
	assert.NotEmpty(t, result.Skipped)
	assert.NotEmpty(t, result.Skipped[0].Reason)
}

func TestImport_MetaJSONMissingName_ReturnsSkipped(t *testing.T) {
	e := newTestEnv(t)

	meta, _ := json.Marshal(map[string]any{
		// name field intentionally omitted
		"variants": []map[string]any{{
			"size": "1x1", "cols": 40, "rows": 12, "fps": 10,
			"frames_file": "frames-1x1.json",
		}},
	})
	frames, _ := json.Marshal([]string{"<span>f</span>"})

	res := e.postMultipart(t, "/api/ascii/import", map[string][]byte{
		"noname/meta.json":        meta,
		"noname/frames-1x1.json": frames,
	})
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	assert.Empty(t, result.Imported)
	assert.NotEmpty(t, result.Skipped)
}

func TestImport_MissingFramesFile_ReturnsSkipped(t *testing.T) {
	e := newTestEnv(t)

	// meta.json references frames-1x1.json but we don't include it.
	meta, _ := json.Marshal(map[string]any{
		"name": "broken",
		"variants": []map[string]any{{
			"size": "1x1", "cols": 40, "rows": 12, "fps": 10,
			"frames_file": "frames-1x1.json",
		}},
	})

	res := e.postMultipart(t, "/api/ascii/import", map[string][]byte{
		"broken/meta.json": meta,
		// frames-1x1.json intentionally absent
	})
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	var result api.ImportResult
	decodeJSON(t, res.Body, &result)

	assert.Empty(t, result.Imported)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, "broken", result.Skipped[0].Name)
	assert.Contains(t, result.Skipped[0].Reason, "frames-1x1.json")
}

func TestImport_NoFiles_ReturnsSkipped(t *testing.T) {
	e := newTestEnv(t)

	// Send an empty multipart form.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/api/ascii/import", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	// No meta.json found → skipped, but not a 4xx — empty upload is handled gracefully.
	var result api.ImportResult
	decodeJSON(t, res.Body, &result)
	assert.Empty(t, result.Imported)
}

// ── Export tests ──────────────────────────────────────────────────────────────

// seedAnimation inserts an animation directly into the store.
func seedAnimation(t *testing.T, st *store.SQLiteStore, name, size, source string) {
	t.Helper()
	frames, _ := json.Marshal([]string{`<span>f0</span>`, `<span>f1</span>`})
	var frameSlice []string
	json.Unmarshal(frames, &frameSlice) //nolint:errcheck
	gz, first, err := store.CompressFrames(frameSlice)
	require.NoError(t, err)
	err = st.Put(context.Background(), store.AnimationVariant{
		Name:       name,
		Source:     source,
		Size:       size,
		Cols:       40,
		Rows:       12,
		FPS:        10,
		FirstFrame: first,
		FramesGzip: gz,
	})
	require.NoError(t, err)
}

func TestExport_SingleAnimation_HappyPath(t *testing.T) {
	e := newTestEnv(t)
	seedAnimation(t, e.store, "logo", "1x1", "")

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{"logo"},
	})
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/zip", res.Header.Get("Content-Type"))
	assert.Contains(t, res.Header.Get("Content-Disposition"), "logo.zip")

	entries := readZip(t, res.Body)
	paths := zipPaths(entries)

	assert.Contains(t, paths, "logo/meta.json")
	assert.Contains(t, paths, "logo/frames-1x1.json")

	// meta.json must be valid and reference the correct animation name.
	metaData := zipFileContent(t, entries, "logo/meta.json")
	var meta store.PackMeta
	require.NoError(t, json.Unmarshal(metaData, &meta))
	assert.Equal(t, "logo", meta.Name)
	assert.Len(t, meta.Variants, 1)
	assert.Equal(t, "1x1", meta.Variants[0].Size)
	assert.Equal(t, "frames-1x1.json", meta.Variants[0].FramesFile)

	// frames file must be valid JSON array.
	framesData := zipFileContent(t, entries, "logo/frames-1x1.json")
	var frameSlice []string
	require.NoError(t, json.Unmarshal(framesData, &frameSlice))
	assert.NotEmpty(t, frameSlice)
}

func TestExport_MultipleAnimations_PackFormat(t *testing.T) {
	e := newTestEnv(t)
	seedAnimation(t, e.store, "logo", "1x1", "")
	seedAnimation(t, e.store, "spinner", "1x1", "")

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"name":       "my-pack",
		"version":    "1.0.0",
		"author":     "testuser",
		"animations": []string{"logo", "spinner"},
	})
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, res.Header.Get("Content-Disposition"), "my-pack.zip")

	entries := readZip(t, res.Body)
	paths := zipPaths(entries)

	// Must have a pack.json at the root level of the zip.
	assert.Contains(t, paths, "my-pack/pack.json")
	assert.Contains(t, paths, "my-pack/logo/meta.json")
	assert.Contains(t, paths, "my-pack/logo/frames-1x1.json")
	assert.Contains(t, paths, "my-pack/spinner/meta.json")
	assert.Contains(t, paths, "my-pack/spinner/frames-1x1.json")

	// pack.json must list both animations.
	packData := zipFileContent(t, entries, "my-pack/pack.json")
	var pack store.PackJSON
	require.NoError(t, json.Unmarshal(packData, &pack))
	assert.Equal(t, "my-pack", pack.Name)
	assert.Equal(t, "1.0.0", pack.Version)
	assert.Equal(t, "testuser", pack.Author)
	assert.ElementsMatch(t, []string{"logo", "spinner"}, pack.Animations)
}

func TestExport_MultiVariant_AllSizesPresent(t *testing.T) {
	e := newTestEnv(t)
	seedAnimation(t, e.store, "wave", "1x1", "")
	seedAnimation(t, e.store, "wave", "2x1", "")

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{"wave"},
	})
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)

	entries := readZip(t, res.Body)
	paths := zipPaths(entries)

	assert.Contains(t, paths, "wave/meta.json")
	assert.Contains(t, paths, "wave/frames-1x1.json")
	assert.Contains(t, paths, "wave/frames-2x1.json")

	metaData := zipFileContent(t, entries, "wave/meta.json")
	var meta store.PackMeta
	require.NoError(t, json.Unmarshal(metaData, &meta))
	assert.Len(t, meta.Variants, 2)
}

func TestExport_EmptyAnimationsList_Returns400(t *testing.T) {
	e := newTestEnv(t)

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{},
	})
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestExport_NonExistentAnimation_Returns404(t *testing.T) {
	e := newTestEnv(t)

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{"does-not-exist"},
	})
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestExport_RemoteAnimation_Returns400(t *testing.T) {
	e := newTestEnv(t)
	// Seed a remote animation (source is non-empty).
	seedAnimation(t, e.store, "remote-anim", "1x1", "https://github.com/someone/repo")

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{"remote-anim"},
	})
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)

	var body map[string]string
	decodeJSON(t, res.Body, &body)
	assert.Contains(t, body["error"], "remote-anim")
}

func TestExport_MixedLocalAndRemote_Returns400(t *testing.T) {
	e := newTestEnv(t)
	seedAnimation(t, e.store, "local-anim", "1x1", "")
	seedAnimation(t, e.store, "remote-anim", "1x1", "https://github.com/someone/repo")

	res := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{"local-anim", "remote-anim"},
	})
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

// ── Round-trip test ───────────────────────────────────────────────────────────

// TestImportExportRoundtrip exports an animation, then re-imports the zip
// back in and verifies the data survives the cycle intact.
func TestImportExportRoundtrip(t *testing.T) {
	e := newTestEnv(t)

	// Step 1: import an animation via the import endpoint.
	importRes := e.postMultipart(t, "/api/ascii/import", singleAnimFiles("original"))
	importRes.Body.Close()
	require.Equal(t, http.StatusOK, importRes.StatusCode)

	// Step 2: export it as a zip.
	exportRes := e.postJSON(t, "/api/ascii/export", map[string]any{
		"animations": []string{"original"},
	})
	require.Equal(t, http.StatusOK, exportRes.StatusCode)
	zipEntries := readZip(t, exportRes.Body)
	exportRes.Body.Close()

	// Step 3: reconstruct a file map from the zip and re-import under a new name.
	// We rename the animation by rewriting meta.json so names don't collide.
	newMeta := map[string]any{
		"name":    "roundtripped",
		"palette": map[string]string{"fg": "#cccccc"},
		"variants": []map[string]any{{
			"size": "1x1", "cols": 40, "rows": 12, "fps": 10,
			"frames_file": "frames-1x1.json",
		}},
	}
	newMetaJSON, _ := json.Marshal(newMeta)

	// Pull frames from the exported zip.
	framesData := zipFileContent(t, zipEntries, "original/frames-1x1.json")

	reimportFiles := map[string][]byte{
		"roundtripped/meta.json":        newMetaJSON,
		"roundtripped/frames-1x1.json": framesData,
	}

	importRes2 := e.postMultipart(t, "/api/ascii/import", reimportFiles)
	importRes2.Body.Close()
	require.Equal(t, http.StatusOK, importRes2.StatusCode)

	// Both animations must exist in the store.
	_, err := e.store.Get(context.Background(), "original")
	assert.NoError(t, err)
	_, err = e.store.Get(context.Background(), "roundtripped")
	assert.NoError(t, err)
}
