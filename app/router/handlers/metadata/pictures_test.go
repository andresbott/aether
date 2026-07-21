package metadata_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	metaHandler "github.com/andresbott/aether/app/router/handlers/metadata"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/coverart"
	"github.com/andresbott/aether/internal/metadataedit"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
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
}

func (s stubCoverArt) List(context.Context, string, string) ([]coverart.CoverImage, error) {
	return s.images, nil
}
func (s stubCoverArt) DownloadImage(context.Context, string) ([]byte, string, error) {
	return s.downloadData, s.downloadExt, nil
}

func newPictureHandler(t *testing.T, libRoot string, ca metaHandler.CoverArtClient) (*store.Store, *mux.Router, *model.Library, *assetstore.Store) {
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
	h := &metaHandler.Handler{Store: s, Reader: nullReader{}, Assets: as, CoverArt: ca}
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
