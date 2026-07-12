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

func newCoverHandler(t *testing.T, libRoot string, ca metaHandler.CoverArtClient) (*store.Store, *mux.Router, *model.Library, *assetstore.Store) {
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

func TestCurrentCover_ServesFolderFile(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)

	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=album"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatal("served bytes differ from cover.png")
	}
}

func TestCurrentCover_ServesManagedStoreCover(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A track file with no embedded art and no folder cover image: the only cover
	// is the one saved to the managed store (db target).
	trackAbs := filepath.Join(albumDir, "01.flac")
	if err := os.WriteFile(trackAbs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newCoverHandler(t, root, nil)

	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "01.flac", FilePath: trackAbs})
	if err := as.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=album"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatal("served bytes are not the managed-store cover")
	}
}

type coverInfoBody struct {
	Sources []struct {
		Source string `json:"source"`
		Detail string `json:"detail"`
	} `json:"sources"`
}

func fetchCoverInfo(t *testing.T, r *mux.Router, libID uint) coverInfoBody {
	t.Helper()
	url := "/metadata/cover/info?library_id=" + strconv.FormatUint(uint64(libID), 10) + "&path=album"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body coverInfoBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCoverInfo_ListsAllPresentSources(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A folder cover file.
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// A track scanned into a DB album, plus a managed-store (db) cover.
	trackAbs := filepath.Join(albumDir, "01.flac")
	if err := os.WriteFile(trackAbs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newCoverHandler(t, root, nil)
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "01.flac", FilePath: trackAbs})
	if err := as.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	body := fetchCoverInfo(t, r, lib.ID)
	if len(body.Sources) != 2 {
		t.Fatalf("want db + folder sources, got %+v", body.Sources)
	}
	if body.Sources[0].Source != "db" {
		t.Fatalf("want db first, got %+v", body.Sources)
	}
	if body.Sources[1].Source != "folder" || body.Sources[1].Detail != "cover.png" {
		t.Fatalf("want folder(cover.png) second, got %+v", body.Sources)
	}
}

func TestCoverInfo_NoCover(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)
	body := fetchCoverInfo(t, r, lib.ID)
	if len(body.Sources) != 0 {
		t.Fatalf("expected no sources, got %+v", body.Sources)
	}
}

func TestDeleteCover_Folder(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	coverFile := filepath.Join(albumDir, "cover.png")
	if err := os.WriteFile(coverFile, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)

	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=album&source=folder"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(coverFile); !os.IsNotExist(err) {
		t.Fatal("cover file was not removed")
	}
}

func TestDeleteCover_DB(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	if err := os.WriteFile(trackAbs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newCoverHandler(t, root, nil)
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "01.flac", FilePath: trackAbs})
	key := strconv.FormatUint(uint64(album.ID), 10)
	if err := as.PutManual(assetstore.KindAlbum, key, "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=album&source=db"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, ok := as.Get(assetstore.KindAlbum, key); ok {
		t.Fatal("managed-store cover was not removed")
	}
}

func TestCurrentCover_BySource(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	folderBytes := []byte("FOLDER-COVER-BYTES")
	if err := os.WriteFile(filepath.Join(albumDir, "cover.png"), folderBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	trackAbs := filepath.Join(albumDir, "01.flac")
	if err := os.WriteFile(trackAbs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newCoverHandler(t, root, nil)
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "01.flac", FilePath: trackAbs})
	if err := as.PutManual(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10), "png", pngBytes); err != nil {
		t.Fatal(err)
	}

	get := func(source string) *httptest.ResponseRecorder {
		url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=album&source=" + source
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		return w
	}
	if w := get("folder"); !bytes.Equal(w.Body.Bytes(), folderBytes) {
		t.Fatalf("source=folder served wrong bytes: %q", w.Body.Bytes())
	}
	if w := get("db"); !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Fatal("source=db served wrong bytes")
	}
	if w := get("embedded"); w.Code != http.StatusNotFound {
		t.Fatalf("source=embedded should 404 (no embedded art), got %d", w.Code)
	}
}

func TestDeleteCover_EmbeddedSelectedPaths(t *testing.T) {
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
	if err := metadataedit.WriteEmbeddedCover(one, pngBytes); err != nil {
		t.Fatal(err)
	}
	if err := metadataedit.WriteEmbeddedCover(two, pngBytes); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)

	// Delete embedded only from the selected track 01.flac.
	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) +
		"&path=album&source=embedded&paths=album/01.flac"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	if img, _ := taglib.ReadImage(one); len(img) != 0 {
		t.Fatal("01.flac embedded cover was not removed")
	}
	if img, _ := taglib.ReadImage(two); len(img) == 0 {
		t.Fatal("02.flac embedded cover was removed but should have been kept")
	}
}

func TestDeleteCover_InvalidSource(t *testing.T) {
	_, r, lib, _ := newCoverHandler(t, t.TempDir(), nil)
	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=&source=bogus"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", url, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCurrentCover_NotFound(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)
	url := "/metadata/cover?library_id=" + strconv.FormatUint(uint64(lib.ID), 10) + "&path=album"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestCoverCandidates(t *testing.T) {
	ca := stubCoverArt{images: []coverart.CoverImage{
		{ID: "1", ImageURL: "http://img/f.jpg", ThumbURL: "http://img/f-250.jpg", IsFront: true},
	}}
	_, r, _, _ := newCoverHandler(t, t.TempDir(), ca)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/cover/candidates?mbid=rel-1", nil))
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

func TestCoverCandidates_RequiresMBID(t *testing.T) {
	_, r, _, _ := newCoverHandler(t, t.TempDir(), stubCoverArt{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metadata/cover/candidates", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// buildCoverForm builds a multipart body for POST /metadata/cover with an
// uploaded image file.
func buildCoverForm(t *testing.T, libID uint, target string, paths []string, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(libID), 10))
	_ = mw.WriteField("target", target)
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

func TestApplyCover_Folder(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)

	body, ct := buildCoverForm(t, lib.ID, "folder", []string{"album/01.flac"}, "art.png", pngBytes)
	req := httptest.NewRequest("POST", "/metadata/cover", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(albumDir, "cover.png"))
	if err != nil {
		t.Fatalf("cover.png not written: %v", err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Fatal("written cover differs")
	}
}

func TestApplyCover_DB(t *testing.T) {
	root := t.TempDir()
	albumDir := filepath.Join(root, "album")
	if err := os.Mkdir(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s, r, lib, as := newCoverHandler(t, root, nil)

	// Seed a DB album + track whose FilePath matches the resolved path.
	album := model.Album{Name: "X", NameNorm: "x", AlbumArtistNorm: "y"}
	s.DB().Create(&album)
	trackAbs := filepath.Join(albumDir, "01.flac")
	s.DB().Create(&model.Track{AlbumID: album.ID, LibraryID: lib.ID, Filename: "01.flac", FilePath: trackAbs})

	body, ct := buildCoverForm(t, lib.ID, "db", []string{"album/01.flac"}, "art.png", pngBytes)
	req := httptest.NewRequest("POST", "/metadata/cover", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	p, ok := as.Get(assetstore.KindAlbum, strconv.FormatUint(uint64(album.ID), 10))
	if !ok {
		t.Fatal("cover not stored in asset store")
	}
	stored, _ := os.ReadFile(p)
	if !bytes.Equal(stored, pngBytes) {
		t.Fatal("stored cover differs")
	}
}

func TestApplyCover_DB_NoAlbum(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, r, lib, _ := newCoverHandler(t, root, nil)

	body, ct := buildCoverForm(t, lib.ID, "db", []string{"album/01.flac"}, "art.png", pngBytes)
	req := httptest.NewRequest("POST", "/metadata/cover", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 when album not scanned, got %d: %s", w.Code, w.Body.String())
	}
}

func TestApplyCover_Embedded(t *testing.T) {
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

	_, r, lib, _ := newCoverHandler(t, root, nil)

	body, ct := buildCoverForm(t, lib.ID, "embedded", []string{"album/01.flac"}, "art.png", pngBytes)
	req := httptest.NewRequest("POST", "/metadata/cover", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	img, err := taglib.ReadImage(dst)
	if err != nil || len(img) == 0 {
		t.Fatalf("embedded image not written: err=%v len=%d", err, len(img))
	}
}

func TestApplyCover_InvalidTarget(t *testing.T) {
	_, r, lib, _ := newCoverHandler(t, t.TempDir(), nil)
	body, ct := buildCoverForm(t, lib.ID, "bogus", []string{"album/01.flac"}, "art.png", pngBytes)
	req := httptest.NewRequest("POST", "/metadata/cover", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
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
