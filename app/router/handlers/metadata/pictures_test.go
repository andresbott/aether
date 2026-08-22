package metadata_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/httperr"
	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/coverart"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/upstream"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"go.senan.xyz/taglib"
	"gorm.io/gorm"
)

// a minimal 1x1 PNG
var pngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

type stubCoverArt struct {
	images       []coverart.CoverImage
	downloadData []byte
	downloadExt  string
	err          error
}

func (s stubCoverArt) List(context.Context, string, string) ([]coverart.CoverImage, error) {
	return s.images, s.err
}
func (s stubCoverArt) DownloadImage(context.Context, string) ([]byte, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return s.downloadData, s.downloadExt, nil
}

func newPictureHandler(t *testing.T, libRoot string, ca metaHandler.CoverArtClient) (*store.Store, *mux.Router, *model.Library) {
	t.Helper()
	return newPictureHandlerWithRescan(t, libRoot, ca, nil)
}

func newPictureHandlerWithRescan(
	t *testing.T, libRoot string, ca metaHandler.CoverArtClient, rs metaHandler.TrackRescanner,
) (*store.Store, *mux.Router, *model.Library) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: libRoot, FollowSymlinks: true}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}, CoverArt: ca, Rescan: rs}
	r := mux.NewRouter()
	h.Routes(r)
	return s, r, lib
}

func libIDStr(lib *model.Library) string {
	return strconv.FormatUint(uint64(lib.ID), 10)
}

// seedAlbum creates a DB album with one track at trackAbs and returns the
// album ID.
func seedAlbum(t *testing.T, s *store.Store, lib *model.Library, trackAbs string) string {
	t.Helper()
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: filepath.Base(trackAbs), FilePath: trackAbs})
	return strconv.FormatUint(uint64(album.ID), 10)
}

// pictureSlotBody is one type+slot cell of an inventory response.
type pictureSlotBody struct {
	Slot         string `json:"slot"`
	Detail       string `json:"detail"`
	Mixed        bool   `json:"mixed"`
	PresentCount int    `json:"present_count"`
	TotalCount   int    `json:"total_count"`
	Image        struct {
		URL      string `json:"url"`
		ThumbURL string `json:"thumb_url"`
	} `json:"image"`
}

type picturesBody struct {
	Pictures []struct {
		Type  string            `json:"type"`
		Slots []pictureSlotBody `json:"slots"`
	} `json:"pictures"`
}

// fetchPictures POSTs a picture-selection inventory request and decodes the
// response, asserting 200 (a test that needs a non-200 status builds the
// request itself instead of using this helper).
func fetchPictures(t *testing.T, r *mux.Router, libID uint, paths []string) picturesBody {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"library_id": libID, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/metadata/pictures/inventory", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body picturesBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

// postRemovals POSTs a picture-removal request: library_id, paths[], type and
// slot all travel in the JSON body — never the URL. Does not itself assert a
// status, unlike fetchPictures: several callers below check non-200 outcomes.
func postRemovals(t *testing.T, r *mux.Router, libID uint, paths []string, pictureType, slot string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"library_id": libID,
		"paths":      paths,
		"type":       pictureType,
		"slot":       slot,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/metadata/pictures/removals", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestInventory_MatrixListsPresentSlots(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Folder art: front cover + back cover files.
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "back.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// One track with an embedded Media picture.
	trackAbs := filepath.Join(albumDir, "01.flac")
	copyFixture(t, src, trackAbs)
	if err := metadataedit.WriteEmbeddedPicture(trackAbs, "Media", pngBytes, ""); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body := fetchPictures(t, r, lib.ID, []string{"album/01.flac"})
	if sl, ok := findSlot(body, "Front Cover", "folder"); !ok || sl.Detail != "cover.png" {
		t.Fatalf("front cover folder slot: %+v", body.Pictures)
	}
	if sl, ok := findSlot(body, "Back Cover", "folder"); !ok || sl.Detail != "back.jpg" {
		t.Fatalf("back cover folder slot: %+v", body.Pictures)
	}
	sl, ok := findSlot(body, "Media", "embedded")
	if !ok || sl.PresentCount != 1 || sl.TotalCount != 1 {
		t.Fatalf("media embedded slot: %+v", body.Pictures)
	}
	if sl.Image.URL == "" {
		t.Fatal("embedded slot must carry an image URL")
	}
	// Types with nothing anywhere are omitted.
	if _, ok := findSlot(body, "Artist", "embedded"); ok {
		t.Fatalf("absent type must not be listed: %+v", body.Pictures)
	}
}

// mkDiscDirs creates root/album/CD 1 and root/album/CD 2 and returns them.
func mkDiscDirs(t *testing.T, root string) (string, string) {
	t.Helper()
	one := filepath.Join(root, "album", "CD 1")
	two := filepath.Join(root, "album", "CD 2")
	for _, d := range []string{one, two} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return one, two
}

// A multi-disc album selected as a whole reports its folder art across every
// directory the selection spans, not just the requested path.
func TestInventory_FolderSlotSpansSelectionDirectories(t *testing.T) {
	root := t.TempDir()
	one, _ := mkDiscDirs(t, root)
	if err := os.WriteFile(filepath.Join(one, "back.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body := fetchPictures(t, r, lib.ID, []string{"album/CD 1/01.flac", "album/CD 2/01.flac"})
	sl, ok := findSlot(body, "Back Cover", "folder")
	if !ok {
		t.Fatalf("back cover folder slot not reported: %+v", body.Pictures)
	}
	if sl.Detail != "back.jpg" {
		t.Errorf("detail = %q, want back.jpg", sl.Detail)
	}
	// Only CD 1 holds the file, so the album's folder art is not uniform.
	if !sl.Mixed {
		t.Error("slot should be marked mixed when one disc folder lacks the picture")
	}
}

// findSlot returns one type+slot cell from an inventory response body, or
// ok=false when that type+slot is absent.
func findSlot(body picturesBody, pictureType, slot string) (pictureSlotBody, bool) {
	for _, p := range body.Pictures {
		if p.Type != pictureType {
			continue
		}
		for _, sl := range p.Slots {
			if sl.Slot == slot {
				return sl, true
			}
		}
	}
	return pictureSlotBody{}, false
}

func TestInventory_FolderSlotMixedOnlyWhenContentsDiffer(t *testing.T) {
	jpgBytes := []byte("A-DIFFERENT-IMAGE")
	cases := []struct {
		name      string
		twoBytes  []byte
		wantMixed bool
	}{
		{"identical art in both folders", pngBytes, false},
		{"different art per folder", jpgBytes, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			one, two := mkDiscDirs(t, root)
			if err := os.WriteFile(filepath.Join(one, "back.jpg"), pngBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(two, "back.jpg"), tc.twoBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			_, r, lib := newPictureHandler(t, root, nil)

			body := fetchPictures(t, r, lib.ID, []string{"album/CD 1/01.flac", "album/CD 2/01.flac"})
			sl, ok := findSlot(body, "Back Cover", "folder")
			if !ok {
				t.Fatalf("back cover folder slot not reported: %+v", body.Pictures)
			}
			if sl.Mixed != tc.wantMixed {
				t.Errorf("mixed = %v, want %v", sl.Mixed, tc.wantMixed)
			}
		})
	}
}

// TestInventory_EmptyFolderReturnsNoPictures confirms an album whose
// selection resolves to an empty folder (nothing embedded, nothing in the
// folder) reports no pictures. A bare directory entry in paths[] seeds a
// folder-only album (Dirs=[album], Tracks=nil, per ResolveAlbum) — the
// closest the body-only inventory endpoint has to "browse an empty folder"
// now that there is no separate path= param to fall back on.
func TestInventory_EmptyFolderReturnsNoPictures(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)
	body := fetchPictures(t, r, lib.ID, []string{"album"})
	if len(body.Pictures) != 0 {
		t.Fatalf("expected no pictures, got %+v", body.Pictures)
	}
}

func TestPictureImage_ServesFolderFileByType(t *testing.T) {
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture at %s: %v", fx, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	backBytes := []byte("BACK-COVER-BYTES")
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "back.jpg"), backBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// A real track with no embedded picture of its own, for the "empty
	// embedded slot" case below.
	copyFixture(t, fx, filepath.Join(albumDir, "01.flac"))
	_, r, lib := newPictureHandler(t, root, nil)

	get := func(file, typ, slot string) *httptest.ResponseRecorder {
		url := "/metadata/pictures/image?library_id=" + libIDStr(lib) + "&file=" + file + "&slot=" + slot + "&type=" + typ
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		return w
	}
	if w := get("album%2Fcover.png", "Front%20Cover", "folder"); !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatalf("front folder served wrong bytes: %q", w.Body.Bytes())
	}
	if w := get("album%2Fback.jpg", "Back%20Cover", "folder"); !bytes.Equal(w.Body.Bytes(), backBytes) {
		t.Fatalf("back folder served wrong bytes: %q", w.Body.Bytes())
	}
	if w := get("album%2Fnonexistent.png", "Media", "folder"); w.Code != http.StatusNotFound {
		t.Fatalf("nonexistent folder file should 404, got %d", w.Code)
	}
	if w := get("album%2F01.flac", "Front%20Cover", "embedded"); w.Code != http.StatusNotFound {
		t.Fatalf("empty embedded slot should 404, got %d", w.Code)
	}
}

// TestPictureImage_InvalidTypeAndSlot confirms type/slot validation happens
// independently of file resolution: a bogus type or slot is well-formed but
// invalid input (422) even with no file named at all.
func TestPictureImage_InvalidTypeAndSlot(t *testing.T) {
	_, r, lib := newPictureHandler(t, t.TempDir(), nil)
	base := "/metadata/pictures/image?library_id=" + libIDStr(lib)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", base+"&slot=folder&type=Bogus", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown type: want 422, got %d", w.Code)
	}
	var typeProblem httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &typeProblem); err != nil {
		t.Fatal(err)
	}
	if len(typeProblem.Errors) == 0 || typeProblem.Errors[0].Pointer != "/type" {
		t.Fatalf("expected a /type field error, got %+v", typeProblem.Errors)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", base+"&slot=bogus", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown slot: want 422, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", base+"&slot=db", nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("db slot: want 422, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "slot must be one of embedded, folder") {
		t.Fatalf("db slot error message: %s", w.Body.String())
	}
	var slotProblem httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &slotProblem); err != nil {
		t.Fatal(err)
	}
	if len(slotProblem.Errors) == 0 || slotProblem.Errors[0].Pointer != "/slot" {
		t.Fatalf("expected a /slot field error, got %+v", slotProblem.Errors)
	}
}

// buildPictureForm builds a multipart body for POST /metadata/pictures with an
// uploaded image file. An empty pictureType omits the field (defaults server-side).
func buildPictureForm(t *testing.T, libID uint, slot, pictureType string, paths []string, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(libID), 10))
	_ = mw.WriteField("slot", slot)
	if pictureType != "" {
		_ = mw.WriteField("type", pictureType)
	}
	for _, p := range paths {
		_ = mw.WriteField("paths", p)
	}
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func postPicture(t *testing.T, r *mux.Router, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/metadata/pictures", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestApplyPicture_FolderByType(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "folder", "Back Cover", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(albumDir, "back.png"))
	if err != nil {
		t.Fatalf("back.png not written: %v", err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Fatal("written picture differs")
	}
}

// A folder-slot save for a multi-disc album writes the same file into every
// directory the selected tracks live in (option A: duplicate per folder).
func TestApplyPicture_FolderWritesEverySelectionDirectory(t *testing.T) {
	root := t.TempDir()
	one, two := mkDiscDirs(t, root)
	_, r, lib := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "folder", "Back Cover",
		[]string{"album/CD 1/01.flac", "album/CD 1/02.flac", "album/CD 2/01.flac"},
		"art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	for _, dir := range []string{one, two} {
		got, err := os.ReadFile(filepath.Join(dir, "back.png"))
		if err != nil {
			t.Fatalf("back.png not written in %s: %v", dir, err)
		}
		if !bytes.Equal(got, pngBytes) {
			t.Errorf("written picture differs in %s", dir)
		}
	}
}

func TestApplyPicture_DefaultTypeIsFrontCover(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "folder", "", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(albumDir, "cover.png")); err != nil {
		t.Fatalf("cover.png not written: %v", err)
	}
}

func TestApplyPicture_EmbeddedByType(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(albumDir, "01.flac")
	copyFixture(t, src, dst)
	_, r, lib := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "embedded", "Back Cover", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	img, ok, err := metadataedit.ReadEmbeddedPicture(dst, "Back Cover")
	if err != nil || !ok || len(img) == 0 {
		t.Fatalf("embedded back cover not written: ok=%v err=%v", ok, err)
	}
	if _, ok, _ := metadataedit.ReadEmbeddedPicture(dst, "Front Cover"); ok {
		t.Fatal("no front cover should exist")
	}
}

func TestApplyPicture_InvalidTargetAndType(t *testing.T) {
	_, r, lib := newPictureHandler(t, t.TempDir(), nil)
	body, ct := buildPictureForm(t, lib.ID, "bogus", "", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad target: want 422, got %d", w.Code)
	}
	body, ct = buildPictureForm(t, lib.ID, "db", "", []string{"album/01.flac"}, "art.png", pngBytes)
	w := postPicture(t, r, body, ct)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("db target: want 422, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "slot must be one of embedded, folder") {
		t.Fatalf("db target error message: %s", w.Body.String())
	}
	var slotProblem httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &slotProblem); err != nil {
		t.Fatal(err)
	}
	if len(slotProblem.Errors) == 0 || slotProblem.Errors[0].Pointer != "/slot" {
		t.Fatalf("expected a /slot field error, got %+v", slotProblem.Errors)
	}
	body, ct = buildPictureForm(t, lib.ID, "folder", "Bogus Type", []string{"album/01.flac"}, "art.png", pngBytes)
	w = postPicture(t, r, body, ct)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad type: want 422, got %d", w.Code)
	}
	var typeProblem httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &typeProblem); err != nil {
		t.Fatal(err)
	}
	if len(typeProblem.Errors) == 0 || typeProblem.Errors[0].Pointer != "/type" {
		t.Fatalf("expected a /type field error, got %+v", typeProblem.Errors)
	}
}

func TestRemovals_FolderByType(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	coverFile := filepath.Join(albumDir, "cover.png")
	backFile := filepath.Join(albumDir, "back.jpg")
	if err := os.WriteFile(coverFile, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backFile, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	// "album" itself, a bare directory entry: ResolveAlbum seeds a
	// folder-only album from it (Dirs=[albumDir], Tracks=nil) — there is no
	// separate track file here, and none is needed for a folder-slot removal.
	w := postRemovals(t, r, lib.ID, []string{"album"}, "Back Cover", "folder")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(backFile); !os.IsNotExist(err) {
		t.Fatal("back.jpg was not removed")
	}
	if _, err := os.Stat(coverFile); err != nil {
		t.Fatal("cover.png must survive deleting the back cover")
	}
}

// Removing folder art for a multi-disc album deletes it from every directory
// the selection spans, mirroring the fan-out on save.
func TestRemovals_FolderRemovesEverySelectionDirectory(t *testing.T) {
	root := t.TempDir()
	one, two := mkDiscDirs(t, root)
	for _, dir := range []string{one, two} {
		if err := os.WriteFile(filepath.Join(dir, "back.jpg"), pngBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cover.png"), pngBytes, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, r, lib := newPictureHandler(t, root, nil)

	w := postRemovals(t, r, lib.ID,
		[]string{"album/CD 1/01.flac", "album/CD 2/01.flac"}, "Back Cover", "folder")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	for _, dir := range []string{one, two} {
		if _, err := os.Stat(filepath.Join(dir, "back.jpg")); !os.IsNotExist(err) {
			t.Errorf("back.jpg survived in %s", dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "cover.png")); err != nil {
			t.Errorf("cover.png must survive in %s: %v", dir, err)
		}
	}
}

func TestRemovals_EmbeddedSelectedPathsAndType(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	one := filepath.Join(albumDir, "01.flac")
	two := filepath.Join(albumDir, "02.flac")
	copyFixture(t, src, one)
	copyFixture(t, src, two)
	for _, p := range []string{one, two} {
		if err := metadataedit.WriteEmbeddedPicture(p, "Front Cover", pngBytes, ""); err != nil {
			t.Fatal(err)
		}
		if err := metadataedit.WriteEmbeddedPicture(p, "Back Cover", pngBytes, ""); err != nil {
			t.Fatal(err)
		}
	}
	_, r, lib := newPictureHandler(t, root, nil)

	// Delete the embedded back cover only from the selected track 01.flac.
	w := postRemovals(t, r, lib.ID, []string{"album/01.flac"}, "Back Cover", "embedded")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	if _, ok, _ := metadataedit.ReadEmbeddedPicture(one, "Back Cover"); ok {
		t.Fatal("01.flac back cover was not removed")
	}
	if _, ok, _ := metadataedit.ReadEmbeddedPicture(one, "Front Cover"); !ok {
		t.Fatal("01.flac front cover must survive a back-cover delete")
	}
	if _, ok, _ := metadataedit.ReadEmbeddedPicture(two, "Back Cover"); !ok {
		t.Fatal("02.flac back cover was removed but should have been kept")
	}
}

func TestRemovals_InvalidSlot(t *testing.T) {
	_, r, lib := newPictureHandler(t, t.TempDir(), nil)
	w := postRemovals(t, r, lib.ID, []string{"a.flac"}, "", "bogus")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
	w = postRemovals(t, r, lib.ID, []string{"a.flac"}, "", "db")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("db slot: want 422, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "slot must be one of embedded, folder") {
		t.Fatalf("db slot error message: %s", w.Body.String())
	}
	var problem httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Pointer != "/slot" {
		t.Fatalf("expected a /slot field error, got %+v", problem.Errors)
	}
}

func TestPictureCandidates(t *testing.T) {
	ca := stubCoverArt{images: []coverart.CoverImage{
		{ID: "1", ImageURL: "http://img/f.jpg", ThumbURL: "http://img/f-250.jpg", IsFront: true},
	}}
	_, r, _ := newPictureHandler(t, t.TempDir(), ca)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/candidates?mbid=rel-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != "1" || got[0]["isFront"] != true {
		t.Fatalf("unexpected candidates: %s", w.Body.String())
	}
}

// An upstream failure must reach the UI as a readable sentence naming the
// service — never a Go error string with "status 500" in it.
func TestPictureCandidates_UpstreamErrorIsHumanReadable(t *testing.T) {
	ca := stubCoverArt{err: &upstream.Error{
		Service: "Cover Art Archive",
		Kind:    upstream.KindUnavailable,
		Status:  http.StatusInternalServerError,
	}}
	_, r, _ := newPictureHandler(t, t.TempDir(), ca)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/candidates?mbid=rel-1", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := httperr.Slug(body.Type); got != "upstream_error" {
		t.Errorf("code = %q, want upstream_error", got)
	}
	if !strings.Contains(body.Detail, "Cover Art Archive") ||
		!strings.Contains(body.Detail, "temporarily unavailable") {
		t.Errorf("error is not a human sentence: %q", body.Detail)
	}
	if strings.Contains(body.Detail, "status 500") || strings.Contains(body.Detail, "lookup failed") {
		t.Errorf("error leaks internal wording: %q", body.Detail)
	}
}

// A rate-limited provider answers 429 so the UI can say "wait and retry".
func TestPictureCandidates_RateLimitedReturns429(t *testing.T) {
	ca := stubCoverArt{err: &upstream.Error{
		Service: "Cover Art Archive",
		Kind:    upstream.KindRateLimited,
		Status:  http.StatusTooManyRequests,
	}}
	_, r, _ := newPictureHandler(t, t.TempDir(), ca)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/candidates?mbid=rel-1", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if got := httperr.Slug(body.Type); got != "upstream_rate_limited" {
		t.Errorf("code = %q, want upstream_rate_limited", got)
	}
	if !strings.Contains(body.Detail, "too many requests") {
		t.Errorf("unhelpful message: %q", body.Detail)
	}
}

// The image download shares the mapping: staging a Cover Art Archive URL that
// the archive then refuses must not answer with a raw Go error either.
func TestApplyPicture_DownloadUpstreamErrorIsHumanReadable(t *testing.T) {
	root := t.TempDir()
	ca := stubCoverArt{err: &upstream.Error{
		Service: "Cover Art Archive",
		Kind:    upstream.KindUnavailable,
		Status:  http.StatusBadGateway,
	}}
	_, r, lib := newPictureHandler(t, root, ca)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", libIDStr(lib))
	_ = mw.WriteField("slot", "folder")
	_ = mw.WriteField("paths", "album/01.flac")
	_ = mw.WriteField("image_url", "http://img/f.jpg")
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/metadata/pictures", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var body httperr.Problem
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !strings.Contains(body.Detail, "Cover Art Archive") || httperr.Slug(body.Type) != "upstream_error" {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}
}

func TestPictureCandidates_RequiresMBID(t *testing.T) {
	_, r, _ := newPictureHandler(t, t.TempDir(), stubCoverArt{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/candidates", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// TestEmbeddedFrontCover_LegacyReadImage confirms writes through the typed API
// stay readable by taglib.ReadImage (used by subsonic embedded serving).
func TestEmbeddedFrontCover_LegacyReadImage(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(albumDir, "01.flac")
	copyFixture(t, src, dst)
	_, r, lib := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "embedded", "Front Cover", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	img, err := taglib.ReadImage(dst)
	if err != nil || len(img) == 0 {
		t.Fatalf("legacy ReadImage cannot see the typed front cover: err=%v len=%d", err, len(img))
	}
}

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func TestApplyPicture_RescansTheFolderTracks(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "Artist", "Album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	copyFixture(t, src, trackAbs)

	rs := &fakeRescanner{}
	_, r, lib := newPictureHandlerWithRescan(t, root, stubCoverArt{}, rs)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(lib.ID), 10))
	_ = mw.WriteField("slot", "folder")
	_ = mw.WriteField("type", "Front Cover")
	_ = mw.WriteField("paths", "Artist/Album/01.flac")
	part, _ := mw.CreateFormFile("image", "cover.png")
	_, _ = part.Write(pngBytes)
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/metadata/pictures", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(rs.calls) != 1 || len(rs.calls[0]) != 1 || rs.calls[0][0] != trackAbs {
		t.Fatalf("unexpected rescan paths: %v", rs.calls)
	}
	var resp struct {
		Rescan *struct {
			OK bool `json:"ok"`
		} `json:"rescan"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Rescan == nil || !resp.Rescan.OK {
		t.Fatalf("expected rescan ok, got %+v", resp.Rescan)
	}
}

// The picture endpoints never let the user pick the rescan path list: paths[]
// IS the selection now (mandatory, non-empty — there is no more "browse this
// folder with nothing selected" fallback to enumerate it server-side), and the
// frontend's selection comes straight from the editor's own track listing,
// which is wider than the scanner's admission on purpose (extra extensions,
// excludes ignored). This is THE normal path — one .oga sibling or one
// excluded subfolder among the selected paths must not make a correct cover
// removal warn about the index.
func TestRemovals_InadmissibleFolderSiblingsDoNotFailTheRescan(t *testing.T) {
	fx := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(fx); err != nil {
		t.Skipf("no fixture: %v", err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.MkdirAll(filepath.Join(albumDir, "Live"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, fx, filepath.Join(albumDir, "01.flac"))
	copyFixture(t, fx, filepath.Join(albumDir, "02.oga"))       // reader-only extension
	copyFixture(t, fx, filepath.Join(albumDir, "Live/01.flac")) // excluded ancestor dir
	if err := os.WriteFile(filepath.Join(albumDir, "back.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	lib := &model.Library{Name: "Main", Path: root, ExcludePatterns: `["^Live$"]`}
	if err := s.CreateLibrary(lib); err != nil {
		t.Fatal(err)
	}
	h := &metaHandler.Handler{
		Store:  s,
		Reader: wideReader{},
		Rescan: scanner.New(scanner.Config{}, s, wideReader{}),
	}
	r := mux.NewRouter()
	h.Routes(r)

	// The full editor-visible selection for this folder — exactly what the
	// frontend sends (it has no separate "browse this folder" fallback to
	// lean on any more): the admissible track plus the two the scanner will
	// skip.
	w := postRemovals(t, r, lib.ID,
		[]string{"album/01.flac", "album/02.oga", "album/Live/01.flac"}, "Back Cover", "folder")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK     bool `json:"ok"`
		Rescan *struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"rescan"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Rescan == nil || !resp.Rescan.OK {
		t.Fatalf("a correct cover removal must not warn about the index: %+v", resp.Rescan)
	}
}

// A picture delete whose re-index indexed nothing must report ok:false, while
// still answering 200 — the file is already gone from disk.
func TestRemovals_PartialRescanReportsNotOK(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "back.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "01.flac"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs := &fakeRescanner{stats: &scanner.ScanStats{
		TracksProcessed: 0,
		Errors:          []error{errors.New(`read tags "01.flac": broken`)},
	}}
	_, r, lib := newPictureHandlerWithRescan(t, root, stubCoverArt{}, rs)

	w := postRemovals(t, r, lib.ID, []string{"album/01.flac"}, "Back Cover", "folder")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK     bool `json:"ok"`
		Rescan *struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"rescan"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("the delete itself must still report ok: %s", w.Body.String())
	}
	if resp.Rescan == nil || resp.Rescan.OK {
		t.Fatalf("expected rescan not ok, got %+v", resp.Rescan)
	}
	if !strings.Contains(resp.Rescan.Error, "read tags") {
		t.Fatalf("expected the tag-read error, got %q", resp.Rescan.Error)
	}
}

// TestInventory_MalformedPathEntryDegradesGracefully confirms a paths[] mix
// of a valid track and an entry that fails to resolve (escaping the library
// root) still resolves the valid one instead of 500ing. Regression coverage
// for a Task 1 review finding: ResolveAlbum was briefly strict, turning any
// bad paths[] entry into a 500 instead of degrading gracefully like the
// selectionPaths/selectionDirs helpers it replaced.
func TestInventory_MalformedPathEntryDegradesGracefully(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	copyFixture(t, src, trackAbs)
	if err := metadataedit.WriteEmbeddedPicture(trackAbs, "Media", pngBytes, ""); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	// fetchPictures itself asserts status 200, so a regression back to a
	// strict ResolveAlbum (500 on the escaping entry) fails right there.
	body := fetchPictures(t, r, lib.ID, []string{"album/01.flac", "../outside"})
	sl, ok := findSlot(body, "Media", "embedded")
	if !ok || sl.PresentCount != 1 || sl.TotalCount != 1 {
		t.Fatalf("media embedded slot: %+v (want the valid path still resolved despite the escaping entry)", body.Pictures)
	}
}

// TestInventory_AllPathsUnresolvableReturnsEmptyMatrix confirms a paths[]
// selection whose entries are all unresolvable (not merely empty) still
// answers 200 with an empty matrix rather than 500. Unlike the old GET
// endpoint, inventory has no separate browsed-folder param to fall back to —
// paths[] IS the whole selection — so this is the honest new-shape outcome:
// ResolveAlbum's leniency degrades an all-bad selection to "nothing
// resolved", not to a folder nobody named.
func TestInventory_AllPathsUnresolvableReturnsEmptyMatrix(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body := fetchPictures(t, r, lib.ID, []string{"/etc/passwd", "../outside"})
	if len(body.Pictures) != 0 {
		t.Fatalf("expected an empty matrix when every path is unresolvable, got %+v", body.Pictures)
	}
}

// TestPictureImage_RejectsTraversalFile confirms a file that resolves
// outside the library root is rejected outright. Unlike the old
// paths[]-based endpoint (which degraded gracefully when one entry among
// several was bad), pictureImage now addresses exactly one file, so a bad
// one is simply a bad request, not something to fall back from.
func TestPictureImage_RejectsTraversalFile(t *testing.T) {
	_, r, lib := newPictureHandler(t, t.TempDir(), nil)
	url := "/metadata/pictures/image?library_id=" + libIDStr(lib) +
		"&file=..%2Foutside&slot=folder&type=Front%20Cover"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s, want 400 for a file escaping the library root", w.Code, w.Body.String())
	}
}

// TestPictureImage_MalformedFolderFileIs404 confirms an otherwise-valid
// request (known library, valid slot=folder) whose file= is empty (an
// omitted file= — DecodeSource resolves "" to the library root itself,
// since ResolveInLibrary treats an empty relative path as valid) or names a
// directory answers a clean 404. Before OpenSource's folder-branch hardening
// this passed os.Stat (the library root/directory exists) and fell through
// to http.ServeFile on a directory, which redirects (301) before it has
// anything to say about existing — a spurious 301-then-404 instead of the
// documented, direct 404 pictureImage promises for an unresolved source.
func TestPictureImage_MalformedFolderFileIs404(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)
	base := "/metadata/pictures/image?library_id=" + libIDStr(lib) + "&slot=folder&type=Front%20Cover"

	get := func(url string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		return w
	}

	if w := get(base); w.Code != http.StatusNotFound {
		t.Fatalf("omitted file=: status %d, want 404: %s", w.Code, w.Body.String())
	}
	if w := get(base + "&file=album"); w.Code != http.StatusNotFound {
		t.Fatalf("file= a directory: status %d, want 404: %s", w.Code, w.Body.String())
	}
}

// TestInventory_PostBodyReturnsImageURLs is the brief's seed test for the
// production 431 fix: the selection travels in the POST body, never the URL,
// and each populated cell carries a ready-to-render image URL that actually
// serves the picture's bytes.
func TestInventory_PostBodyReturnsImageURLs(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "01.flac"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body := fetchPictures(t, r, lib.ID, []string{"album/01.flac"})
	sl, ok := findSlot(body, "Front Cover", "folder")
	if !ok {
		t.Fatalf("front cover folder slot not reported: %+v", body.Pictures)
	}
	if sl.Image.URL == "" {
		t.Fatal("slot.image.url must not be empty")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", sl.Image.URL, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", sl.Image.URL, w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatalf("image URL served wrong bytes: %q", w.Body.Bytes())
	}
}

// TestInventory_RequiresNonEmptyPaths confirms decodeSelection rejects an
// empty paths[] instead of ResolveAlbum's own (caller-bug-only) error. This is
// well-formed but invalid input, so it answers 422, not 400.
func TestInventory_RequiresNonEmptyPaths(t *testing.T) {
	_, r, lib := newPictureHandler(t, t.TempDir(), nil)
	body := `{"library_id": ` + libIDStr(lib) + `, "paths": []}`
	req := httptest.NewRequest("POST", "/metadata/pictures/inventory", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var problem httperr.ValidationProblem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Pointer != "/paths" {
		t.Fatalf("expected a /paths field error, got %+v", problem.Errors)
	}
}

// TestInventory_RejectsTooManyPaths confirms the maxSelectionPaths cap:
// defense in depth now that the selection travels in a body instead of a
// query string. Well-formed but invalid input, so it answers 422, not 400.
func TestInventory_RejectsTooManyPaths(t *testing.T) {
	_, r, lib := newPictureHandler(t, t.TempDir(), nil)
	paths := make([]string, 51)
	for i := range paths {
		paths[i] = "album/" + strconv.Itoa(i) + ".flac"
	}
	payload, err := json.Marshal(map[string]any{"library_id": lib.ID, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/metadata/pictures/inventory", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for 51 paths, got %d: %s", w.Code, w.Body.String())
	}
}

// TestInventory_UnknownLibrary404 confirms decodeSelection maps a
// gorm.ErrRecordNotFound library lookup to 404, like every other endpoint's
// library_id resolution.
func TestInventory_UnknownLibrary404(t *testing.T) {
	_, r, _ := newPictureHandler(t, t.TempDir(), nil)
	body := `{"library_id": 999, "paths": ["a.flac"]}`
	req := httptest.NewRequest("POST", "/metadata/pictures/inventory", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestInventory_MalformedJSON400 confirms a body that fails to decode 400s
// rather than panicking or 500ing.
func TestInventory_MalformedJSON400(t *testing.T) {
	_, r, _ := newPictureHandler(t, t.TempDir(), nil)
	req := httptest.NewRequest("POST", "/metadata/pictures/inventory", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRemovals_MalformedPathEntryDegradesGracefully mirrors the
// TestInventory_MalformedPathEntryDegradesGracefully case for removals: a
// paths[] mix of a valid entry and one that fails to resolve (escaping the
// library root) still clears the folder art via the valid entry, instead of
// 500ing or rejecting the whole request. Unlike the old query-string
// deletePicture there is no separate "browsed folder" to fall back to when
// EVERY entry is bad (see TestInventory_AllPathsUnresolvableReturnsEmptyMatrix)
// — paths[] is the whole selection now — so this keeps one resolvable entry.
func TestRemovals_MalformedPathEntryDegradesGracefully(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	backFile := filepath.Join(albumDir, "back.jpg")
	if err := os.WriteFile(backFile, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	w := postRemovals(t, r, lib.ID, []string{"album/01.flac", "../outside"}, "Back Cover", "folder")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s (a malformed paths[] entry must degrade to the valid ones, not fail the request)", w.Code, w.Body.String())
	}
	if _, err := os.Stat(backFile); !os.IsNotExist(err) {
		t.Fatal("back.jpg was not removed: a malformed paths[] entry should not block removal via the valid ones")
	}
}

// TestApplyPicture_RejectsUnresolvablePath confirms applyPicture keeps its
// own strict validation of paths[]: unlike the read/delete endpoints above,
// a write must not silently drop a bad path and save fewer files than asked.
// Regression guard for the same fix that made ResolveAlbum lenient —
// applyPicture must not inherit that leniency.
func TestApplyPicture_RejectsUnresolvablePath(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "folder", "Back Cover",
		[]string{"album/01.flac", "../outside"}, "art.png", pngBytes)
	w := postPicture(t, r, body, ct)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s, want 400 for an unresolvable path", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(albumDir, "back.png")); !os.IsNotExist(err) {
		t.Fatal("must not have written anything when one path is invalid")
	}
}

// TestRemovals_PostBodyClearsCell is the brief's seed test for
// POST /metadata/pictures/removals: the selection travels in the POST body
// (never the URL), a removal actually clears the cell — the folder file is
// deleted, the embedded frame is gone — and repeating the same removal
// against the now-empty cell still answers ok:true: clearing something that
// is already gone is a no-op, not an error.
func TestRemovals_PostBodyClearsCell(t *testing.T) {
	src := "../../../../internal/metadataedit/testdata/empty.flac"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no fixture at %s: %v", src, err)
	}
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	backFile := filepath.Join(albumDir, "back.jpg")
	if err := os.WriteFile(backFile, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	copyFixture(t, src, trackAbs)
	if err := metadataedit.WriteEmbeddedPicture(trackAbs, "Front Cover", pngBytes, ""); err != nil {
		t.Fatal(err)
	}
	_, r, lib := newPictureHandler(t, root, nil)

	assertOK := func(w *httptest.ResponseRecorder, label string) {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", label, w.Code, w.Body.String())
		}
		var body struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !body.OK {
			t.Fatalf("%s: expected ok:true, got %s", label, w.Body.String())
		}
	}

	// Folder slot: the file is actually deleted.
	w := postRemovals(t, r, lib.ID, []string{"album/01.flac"}, "Back Cover", "folder")
	assertOK(w, "folder removal")
	if _, err := os.Stat(backFile); !os.IsNotExist(err) {
		t.Fatal("back.jpg was not removed")
	}

	// Repeating it against the now-empty folder cell is still idempotent.
	w = postRemovals(t, r, lib.ID, []string{"album/01.flac"}, "Back Cover", "folder")
	assertOK(w, "repeat folder removal on an empty cell")

	// Embedded slot: the frame is actually gone.
	w = postRemovals(t, r, lib.ID, []string{"album/01.flac"}, "Front Cover", "embedded")
	assertOK(w, "embedded removal")
	if _, ok, _ := metadataedit.ReadEmbeddedPicture(trackAbs, "Front Cover"); ok {
		t.Fatal("embedded front cover was not removed")
	}

	// Repeating it against the now-empty embedded cell is still idempotent.
	w = postRemovals(t, r, lib.ID, []string{"album/01.flac"}, "Front Cover", "embedded")
	assertOK(w, "repeat embedded removal on an empty cell")
}
