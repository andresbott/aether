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

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/assetstore"
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

func newPictureHandler(t *testing.T, libRoot string, ca metaHandler.CoverArtClient) (*store.Store, *mux.Router, *model.Library, *assetstore.Store) {
	t.Helper()
	return newPictureHandlerWithRescan(t, libRoot, ca, nil)
}

func newPictureHandlerWithRescan(
	t *testing.T, libRoot string, ca metaHandler.CoverArtClient, rs metaHandler.TrackRescanner,
) (*store.Store, *mux.Router, *model.Library, *assetstore.Store) {
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
	as := assetstore.New(t.TempDir())
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}, Assets: as, CoverArt: ca, Rescan: rs}
	r := mux.NewRouter()
	h.Routes(r)
	return s, r, lib, as
}

func libIDStr(lib *model.Library) string {
	return strconv.FormatUint(uint64(lib.ID), 10)
}

// seedAlbum creates a DB album with one track at trackAbs and returns the
// album's assetstore key.
func seedAlbum(t *testing.T, s *store.Store, lib *model.Library, trackAbs string) string {
	t.Helper()
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: filepath.Base(trackAbs), FilePath: trackAbs})
	return strconv.FormatUint(uint64(album.ID), 10)
}

type picturesBody struct {
	Pictures []struct {
		Type  string `json:"type"`
		Slots []struct {
			Slot   string `json:"slot"`
			Detail string `json:"detail"`
			Mixed  bool   `json:"mixed"`
		} `json:"slots"`
	} `json:"pictures"`
}

func fetchPictures(t *testing.T, r *mux.Router, libID uint, extra string) picturesBody {
	t.Helper()
	url := "/metadata/pictures?library_id=" + strconv.FormatUint(uint64(libID), 10) + "&path=album" + extra
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body picturesBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPictures_MatrixListsPresentSlots(t *testing.T) {
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
	// A scanned album with a named db picture.
	s, r, lib, as := newPictureHandler(t, root, nil)
	key := seedAlbum(t, s, lib, trackAbs)
	if err := as.PutManualNamed(assetstore.KindAlbum, key, "booklet", "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	body := fetchPictures(t, r, lib.ID, "&paths=album/01.flac")
	got := map[string]map[string]string{}
	for _, p := range body.Pictures {
		got[p.Type] = map[string]string{}
		for _, sl := range p.Slots {
			got[p.Type][sl.Slot] = sl.Detail
		}
	}
	if d, ok := got["Front Cover"]["folder"]; !ok || d != "cover.png" {
		t.Fatalf("front cover folder slot: %+v", got)
	}
	if d, ok := got["Back Cover"]["folder"]; !ok || d != "back.jpg" {
		t.Fatalf("back cover folder slot: %+v", got)
	}
	if d, ok := got["Media"]["embedded"]; !ok || d != "1 of 1 files" {
		t.Fatalf("media embedded slot: %+v", got)
	}
	if _, ok := got["Leaflet Page"]["db"]; !ok {
		t.Fatalf("booklet db slot missing: %+v", got)
	}
	// Types with nothing anywhere are omitted.
	if _, ok := got["Artist"]; ok {
		t.Fatalf("absent type must not be listed: %+v", got)
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
func TestPictures_FolderSlotSpansSelectionDirectories(t *testing.T) {
	root := t.TempDir()
	one, _ := mkDiscDirs(t, root)
	if err := os.WriteFile(filepath.Join(one, "back.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newPictureHandler(t, root, nil)

	body := fetchPictures(t, r, lib.ID, "&paths=album/CD%201/01.flac&paths=album/CD%202/01.flac")
	detail, mixed, ok := findSlot(body, "Back Cover", "folder")
	if !ok {
		t.Fatalf("back cover folder slot not reported: %+v", body.Pictures)
	}
	if detail != "back.jpg" {
		t.Errorf("detail = %q, want back.jpg", detail)
	}
	// Only CD 1 holds the file, so the album's folder art is not uniform.
	if !mixed {
		t.Error("slot should be marked mixed when one disc folder lacks the picture")
	}
}

// findSlot returns one type+slot cell's detail and mixed flag from a matrix body.
func findSlot(body picturesBody, pictureType, slot string) (detail string, mixed bool, ok bool) {
	for _, p := range body.Pictures {
		if p.Type != pictureType {
			continue
		}
		for _, sl := range p.Slots {
			if sl.Slot == slot {
				return sl.Detail, sl.Mixed, true
			}
		}
	}
	return "", false, false
}

func TestPictures_FolderSlotMixedOnlyWhenContentsDiffer(t *testing.T) {
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
			_, r, lib, _ := newPictureHandler(t, root, nil)

			body := fetchPictures(t, r, lib.ID, "&paths=album/CD%201/01.flac&paths=album/CD%202/01.flac")
			_, mixed, ok := findSlot(body, "Back Cover", "folder")
			if !ok {
				t.Fatalf("back cover folder slot not reported: %+v", body.Pictures)
			}
			if mixed != tc.wantMixed {
				t.Errorf("mixed = %v, want %v", mixed, tc.wantMixed)
			}
		})
	}
}

func TestPictures_Empty(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newPictureHandler(t, root, nil)
	body := fetchPictures(t, r, lib.ID, "")
	if len(body.Pictures) != 0 {
		t.Fatalf("expected no pictures, got %+v", body.Pictures)
	}
}

func TestPictureImage_ServesFolderFileByType(t *testing.T) {
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

	get := func(typ, slot string) *httptest.ResponseRecorder {
		url := "/metadata/pictures/image?library_id=" + libIDStr(lib) + "&path=album&slot=" + slot + "&type=" + typ
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		return w
	}
	if w := get("Front%20Cover", "folder"); !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatalf("front folder served wrong bytes: %q", w.Body.Bytes())
	}
	if w := get("Back%20Cover", "folder"); !bytes.Equal(w.Body.Bytes(), backBytes) {
		t.Fatalf("back folder served wrong bytes: %q", w.Body.Bytes())
	}
	if w := get("Media", "folder"); w.Code != http.StatusNotFound {
		t.Fatalf("absent type should 404, got %d", w.Code)
	}
	if w := get("Front%20Cover", "embedded"); w.Code != http.StatusNotFound {
		t.Fatalf("empty embedded slot should 404, got %d", w.Code)
	}
}

// The album's folder art may live in a later disc folder than the primary one
// the request is anchored on; the paths tell the server where else to look.
func TestPictureImage_FolderSlotFindsArtInAnotherDiscFolder(t *testing.T) {
	root := t.TempDir()
	_, two := mkDiscDirs(t, root)
	backBytes := []byte("BACK-COVER-BYTES")
	if err := os.WriteFile(filepath.Join(two, "back.jpg"), backBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newPictureHandler(t, root, nil)

	// path= is CD 1 (the primary folder, which has no art at all).
	url := "/metadata/pictures/image?library_id=" + libIDStr(lib) +
		"&path=album/CD%201&slot=folder&type=Back%20Cover" +
		"&paths=album/CD%201/01.flac&paths=album/CD%202/01.flac"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), backBytes) {
		t.Fatalf("served wrong bytes: %q", w.Body.Bytes())
	}
}

func TestPictureImage_ServesDBSlot(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	if err := os.WriteFile(trackAbs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newPictureHandler(t, root, nil)
	key := seedAlbum(t, s, lib, trackAbs)
	if err := as.PutManualNamed(assetstore.KindAlbum, key, "back", "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	url := "/metadata/pictures/image?library_id=" + libIDStr(lib) + "&path=album&slot=db&type=Back%20Cover"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatalf("db slot: status %d", w.Code)
	}
}

func TestPictureImage_InvalidTypeAndSlot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newPictureHandler(t, root, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/image?library_id="+libIDStr(lib)+"&path=album&slot=folder&type=Bogus", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown type: want 400, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/image?library_id="+libIDStr(lib)+"&path=album&slot=bogus", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown slot: want 400, got %d", w.Code)
	}
}

// buildPictureForm builds a multipart body for POST /metadata/pictures with an
// uploaded image file. An empty pictureType omits the field (defaults server-side).
func buildPictureForm(t *testing.T, libID uint, target, pictureType string, paths []string, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(libID), 10))
	_ = mw.WriteField("target", target)
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

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
	_, r, lib, _ := newPictureHandler(t, root, nil)

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

// The embedded slot already targets the listed files, and the db slot is one
// album-wide entry: neither may be affected by the folder fan-out.
func TestApplyPicture_DBWritesOneEntryForMultiDiscSelection(t *testing.T) {
	root := t.TempDir()
	one, two := mkDiscDirs(t, root)
	s, r, lib, as := newPictureHandler(t, root, nil)
	// Both discs' tracks belong to the same scanned album.
	trackOne := filepath.Join(one, "01.flac")
	key := seedAlbum(t, s, lib, trackOne)
	s.DB().Create(&model.Track{
		AlbumID:   parseUintOrFail(t, key),
		LibraryID: lib.ID,
		Filename:  "01.flac",
		FilePath:  filepath.Join(two, "01.flac"),
	})

	body, ct := buildPictureForm(t, lib.ID, "db", "Back Cover",
		[]string{"album/CD 1/01.flac", "album/CD 2/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, ok := as.GetNamed(assetstore.KindAlbum, key, "back"); !ok {
		t.Fatal("back picture not stored in asset store")
	}
	// No stray art files: the db slot must not touch the music folders.
	for _, dir := range []string{one, two} {
		if _, err := os.Stat(filepath.Join(dir, "back.png")); !os.IsNotExist(err) {
			t.Errorf("db save must not write a folder file in %s", dir)
		}
	}
}

func parseUintOrFail(t *testing.T, s string) uint {
	t.Helper()
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return uint(n)
}

func TestApplyPicture_DefaultTypeIsFrontCover(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "folder", "", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(albumDir, "cover.png")); err != nil {
		t.Fatalf("cover.png not written: %v", err)
	}
}

func TestApplyPicture_DBNamedEntry(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newPictureHandler(t, root, nil)
	trackAbs := filepath.Join(albumDir, "01.flac")
	key := seedAlbum(t, s, lib, trackAbs)

	body, ct := buildPictureForm(t, lib.ID, "db", "Back Cover", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	p, ok := as.GetNamed(assetstore.KindAlbum, key, "back")
	if !ok {
		t.Fatal("back picture not stored in asset store")
	}
	stored, _ := os.ReadFile(p)
	if !bytes.Equal(stored, pngBytes) {
		t.Fatal("stored picture differs")
	}
	// The front-cover entry must not exist.
	if _, ok := as.Get(assetstore.KindAlbum, key); ok {
		t.Fatal("saving a back cover must not create a front-cover entry")
	}
}

func TestApplyPicture_DB_NoAlbum(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newPictureHandler(t, root, nil)

	body, ct := buildPictureForm(t, lib.ID, "db", "Back Cover", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusNotFound {
		t.Fatalf("want 404 when album not scanned, got %d: %s", w.Code, w.Body.String())
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

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
	_, r, lib, _ := newPictureHandler(t, t.TempDir(), nil)
	body, ct := buildPictureForm(t, lib.ID, "bogus", "", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusBadRequest {
		t.Fatalf("bad target: want 400, got %d", w.Code)
	}
	body, ct = buildPictureForm(t, lib.ID, "folder", "Bogus Type", []string{"album/01.flac"}, "art.png", pngBytes)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusBadRequest {
		t.Fatalf("bad type: want 400, got %d", w.Code)
	}
}

func TestDeletePicture_FolderByType(t *testing.T) {
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

	url := "/metadata/pictures?library_id=" + libIDStr(lib) + "&path=album&slot=folder&type=Back%20Cover"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
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
func TestDeletePicture_FolderRemovesEverySelectionDirectory(t *testing.T) {
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

	url := "/metadata/pictures?library_id=" + libIDStr(lib) +
		"&path=album&slot=folder&type=Back%20Cover" +
		"&paths=album/CD%201/01.flac&paths=album/CD%202/01.flac"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
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

func TestDeletePicture_DBLeavesOtherEntries(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	if err := os.WriteFile(trackAbs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newPictureHandler(t, root, nil)
	key := seedAlbum(t, s, lib, trackAbs)
	if err := as.PutManual(assetstore.KindAlbum, key, "png", pngBytes); err != nil {
		t.Fatal(err)
	}
	if err := as.PutManualNamed(assetstore.KindAlbum, key, "back", "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	url := "/metadata/pictures?library_id=" + libIDStr(lib) + "&path=album&slot=db&type=Back%20Cover"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, ok := as.GetNamed(assetstore.KindAlbum, key, "back"); ok {
		t.Fatal("back entry was not removed")
	}
	if _, ok := as.Get(assetstore.KindAlbum, key); !ok {
		t.Fatal("front cover must survive deleting the back entry")
	}
}

func TestDeletePicture_EmbeddedSelectedPathsAndType(t *testing.T) {
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

	// Delete the embedded back cover only from the selected track 01.flac.
	url := "/metadata/pictures?library_id=" + libIDStr(lib) +
		"&path=album&slot=embedded&type=Back%20Cover&paths=album/01.flac"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
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

func TestDeletePicture_InvalidSlot(t *testing.T) {
	_, r, lib, _ := newPictureHandler(t, t.TempDir(), nil)
	url := "/metadata/pictures?library_id=" + libIDStr(lib) + "&path=&slot=bogus"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestPictureCandidates(t *testing.T) {
	ca := stubCoverArt{images: []coverart.CoverImage{
		{ID: "1", ImageURL: "http://img/f.jpg", ThumbURL: "http://img/f-250.jpg", IsFront: true},
	}}
	_, r, _, _ := newPictureHandler(t, t.TempDir(), ca)

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
	_, r, _, _ := newPictureHandler(t, t.TempDir(), ca)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/candidates?mbid=rel-1", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", w.Code)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "upstream_error" {
		t.Errorf("code = %q, want upstream_error", body.Code)
	}
	if !strings.Contains(body.Error, "Cover Art Archive") ||
		!strings.Contains(body.Error, "temporarily unavailable") {
		t.Errorf("error is not a human sentence: %q", body.Error)
	}
	if strings.Contains(body.Error, "status 500") || strings.Contains(body.Error, "lookup failed") {
		t.Errorf("error leaks internal wording: %q", body.Error)
	}
}

// A rate-limited provider answers 429 so the UI can say "wait and retry".
func TestPictureCandidates_RateLimitedReturns429(t *testing.T) {
	ca := stubCoverArt{err: &upstream.Error{
		Service: "Cover Art Archive",
		Kind:    upstream.KindRateLimited,
		Status:  http.StatusTooManyRequests,
	}}
	_, r, _, _ := newPictureHandler(t, t.TempDir(), ca)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/pictures/candidates?mbid=rel-1", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", w.Code)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != "upstream_rate_limited" {
		t.Errorf("code = %q, want upstream_rate_limited", body.Code)
	}
	if !strings.Contains(body.Error, "too many requests") {
		t.Errorf("unhelpful message: %q", body.Error)
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
	_, r, lib, _ := newPictureHandler(t, root, ca)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", libIDStr(lib))
	_ = mw.WriteField("target", "db")
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
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !strings.Contains(body.Error, "Cover Art Archive") || body.Code != "upstream_error" {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}
}

func TestPictureCandidates_RequiresMBID(t *testing.T) {
	_, r, _, _ := newPictureHandler(t, t.TempDir(), stubCoverArt{})
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
	_, r, lib, _ := newPictureHandler(t, root, nil)

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
	_, r, lib, _ := newPictureHandlerWithRescan(t, root, stubCoverArt{}, rs)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(lib.ID), 10))
	_ = mw.WriteField("target", "folder")
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

// The picture endpoints never let the user pick the rescan path list: with no
// explicit paths, selectionPaths -> folderTrackPaths recursively lists every
// file the *editor's* reader accepts under the album dir, which is wider than
// the scanner's admission (extra extensions, excludes ignored). The frontend
// always sends paths: undefined for folder/db slots, so this is THE normal
// path — one .oga sibling or one excluded subfolder must not make a correct
// cover removal warn about the index.
func TestDeletePicture_InadmissibleFolderSiblingsDoNotFailTheRescan(t *testing.T) {
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
		Assets: assetstore.New(t.TempDir()),
		Rescan: scanner.New(scanner.Config{}, s, wideReader{}),
	}
	r := mux.NewRouter()
	h.Routes(r)

	// No paths param — exactly what useEditSession sends for a folder slot.
	url := "/metadata/pictures?library_id=" + libIDStr(lib) + "&path=album&slot=folder&type=Back%20Cover"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
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
func TestDeletePicture_PartialRescanReportsNotOK(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "back.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// selectionPaths falls back to the folder's tracks; nullReader reports none,
	// so pass the path explicitly to give the rescan something to do.
	if err := os.WriteFile(filepath.Join(albumDir, "01.flac"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs := &fakeRescanner{stats: &scanner.ScanStats{
		TracksProcessed: 0,
		Errors:          []error{errors.New(`read tags "01.flac": broken`)},
	}}
	_, r, lib, _ := newPictureHandlerWithRescan(t, root, stubCoverArt{}, rs)

	url := "/metadata/pictures?library_id=" + libIDStr(lib) +
		"&path=album&slot=folder&type=Back%20Cover&paths=album/01.flac"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
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
