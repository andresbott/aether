package metadata_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/artistimage"
)

// mkAlbumTrack creates <root>/<artist>/<album>/a.flac so <artist> is an artist
// folder (an album sub-folder holding a track).
func mkAlbumTrack(t *testing.T, root, artist, album string) {
	t.Helper()
	dir := filepath.Join(root, artist, album)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.flac"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ----- eligibility -----

type imageMetaBody struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Bytes  int64  `json:"bytes"`
}

type artistFolderBody struct {
	Eligible         bool           `json:"eligible"`
	Artist           string         `json:"artist"`
	Path             string         `json:"path"`
	CurrentImage     string         `json:"current_image"`
	CurrentImageMeta *imageMetaBody `json:"current_image_meta"`
}

func fetchArtistFolder(t *testing.T, r http.Handler, libID, path string) (*httptest.ResponseRecorder, artistFolderBody) {
	t.Helper()
	reqURL := "/metadata/artist-folder?library_id=" + libID + "&path=" + url.QueryEscape(path)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", reqURL, nil))
	var body artistFolderBody
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v (%s)", err, w.Body.String())
		}
	}
	return w, body
}

// TestArtistFolder_EligibleWhenAlbumArtistMatches: a folder whose albums are
// tagged with an album artist matching its name is an artist folder.
func TestArtistFolder_EligibleWhenAlbumArtistMatches(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	w, body := fetchArtistFolder(t, r, libIDStr(lib), "Radiohead")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !body.Eligible {
		t.Fatalf("expected eligible=true, got %+v", body)
	}
	if body.Artist != "Radiohead" || body.Path != "Radiohead" {
		t.Errorf("artist=%q path=%q, want Radiohead/Radiohead", body.Artist, body.Path)
	}
	if body.CurrentImage != "" {
		t.Errorf("current_image = %q, want empty", body.CurrentImage)
	}
}

// TestArtistFolder_ReportsExistingImage: an artist image already in the folder is
// reported by filename.
func TestArtistFolder_ReportsExistingImage(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")
	if err := os.WriteFile(filepath.Join(root, "Radiohead", "artist.jpg"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "Radiohead")
	if !body.Eligible || body.CurrentImage != "artist.jpg" {
		t.Errorf("got %+v, want eligible with current_image artist.jpg", body)
	}
}

// TestArtistFolder_ReportsCurrentImageMeta: the current artist image's size,
// dimensions and format ride along with the eligibility response, so the editor
// can show them without a second request.
func TestArtistFolder_ReportsCurrentImageMeta(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")
	if err := os.WriteFile(filepath.Join(root, "Radiohead", "artist.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "Radiohead")
	if body.CurrentImage != "artist.png" {
		t.Fatalf("current_image = %q, want artist.png", body.CurrentImage)
	}
	m := body.CurrentImageMeta
	if m == nil {
		t.Fatal("current_image_meta is nil, want it populated")
	}
	if m.Width != 1 || m.Height != 1 || m.Format != "png" || m.Bytes != int64(len(pngBytes)) {
		t.Errorf("meta = %+v, want 1x1 png %d bytes", m, len(pngBytes))
	}
}

// TestArtistFolder_NoImageMetaWhenAbsent: with no artist image on disk, the
// response carries no image meta.
func TestArtistFolder_NoImageMetaWhenAbsent(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "Radiohead")
	if body.CurrentImageMeta != nil {
		t.Errorf("current_image_meta = %+v, want nil when no image", body.CurrentImageMeta)
	}
}

// TestArtistFolder_NotEligibleWhenNameMismatch: a genre/collection folder whose
// content is tagged with a different album artist is not an artist folder.
func TestArtistFolder_NotEligibleWhenNameMismatch(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Rock", "OK Computer") // folder "Rock", tags say "Radiohead"

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "Rock")
	if body.Eligible {
		t.Errorf("expected eligible=false, got %+v", body)
	}
}

// TestArtistFolder_NotEligibleForRoot: the library root is never an artist folder.
func TestArtistFolder_NotEligibleForRoot(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "")
	if body.Eligible {
		t.Errorf("expected eligible=false for root, got %+v", body)
	}
}

// TestArtistFolder_EligibleWhenSelectingAlbum: selecting an album folder resolves
// up to its artist folder.
func TestArtistFolder_EligibleWhenSelectingAlbum(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "Radiohead/OK Computer")
	if !body.Eligible || body.Path != "Radiohead" || body.Artist != "Radiohead" {
		t.Errorf("got %+v; want eligible resolving to Radiohead", body)
	}
}

// TestArtistFolder_EligibleWhenSelectingDisc: selecting a disc sub-folder resolves
// up to the artist folder two levels above.
func TestArtistFolder_EligibleWhenSelectingDisc(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Radiohead", "OK Computer", "CD 1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.flac"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, nil)

	_, body := fetchArtistFolder(t, r, libIDStr(lib), "Radiohead/OK Computer/CD 1")
	if !body.Eligible || body.Path != "Radiohead" {
		t.Errorf("got %+v; want eligible resolving to Radiohead from a disc", body)
	}
}

// ----- write / serve / delete -----

func buildArtistImageForm(t *testing.T, libID uint, path, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(libID), 10))
	_ = mw.WriteField("path", path)
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

func buildArtistImagePick(t *testing.T, libID uint, path, mbid, imgURL string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(libID), 10))
	_ = mw.WriteField("path", path)
	_ = mw.WriteField("mbid", mbid)
	_ = mw.WriteField("url", imgURL)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func postArtistImage(t *testing.T, r http.Handler, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/metadata/artist-image", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func reqArtistImage(t *testing.T, r http.Handler, method, libID, path string) *httptest.ResponseRecorder {
	t.Helper()
	reqURL := "/metadata/artist-image?library_id=" + libID + "&path=" + url.QueryEscape(path)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, reqURL, nil))
	return w
}

const testMBID = "11111111-1111-1111-1111-111111111111"

// TestSetArtistImage_WritesUploadedFile: an uploaded file lands in the selected
// folder as artist.<ext>.
func TestSetArtistImage_WritesUploadedFile(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil)

	body, ct := buildArtistImageForm(t, lib.ID, "Radiohead", "x.png", pngBytes)
	w := postArtistImage(t, r, body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(root, "Radiohead", "artist.png"))
	if err != nil {
		t.Fatalf("artist.png not written: %v", err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Errorf("artist.png content mismatch")
	}
}

// TestSetArtistImage_RequiresPath: without a folder there is nowhere to write.
func TestSetArtistImage_RequiresPath(t *testing.T) {
	root := t.TempDir()
	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil)
	body, ct := buildArtistImageForm(t, lib.ID, "", "x.png", pngBytes)
	w := postArtistImage(t, r, body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

// TestSetArtistImage_DownloadsOnlinePick: an online pick (mbid + a candidate URL)
// is downloaded from the provider and written as artist.<ext>.
func TestSetArtistImage_DownloadsOnlinePick(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	const imgURL = "https://provider.example/full.jpg"
	fetcher := stubArtistFetcher{
		candidates: []artistimage.ImageCandidate{{FullURL: imgURL, Provider: "fanart"}},
		data:       pngBytes,
		ext:        "jpg",
	}
	_, r, lib := newArtistImageHandler(t, root, nullReader{}, fetcher, nil)

	body, ct := buildArtistImagePick(t, lib.ID, "Radiohead", testMBID, imgURL)
	w := postArtistImage(t, r, body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(root, "Radiohead", "artist.jpg"))
	if err != nil {
		t.Fatalf("artist.jpg not written: %v", err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Errorf("artist.jpg content mismatch")
	}
}

// TestSetArtistImage_RejectsUrlNotInCandidates: the SSRF guard refuses a URL the
// provider did not offer for this MBID.
func TestSetArtistImage_RejectsUrlNotInCandidates(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")
	fetcher := stubArtistFetcher{
		candidates: []artistimage.ImageCandidate{{FullURL: "https://provider.example/a.jpg", Provider: "fanart"}},
		data:       pngBytes,
		ext:        "jpg",
	}
	_, r, lib := newArtistImageHandler(t, root, nullReader{}, fetcher, nil)

	body, ct := buildArtistImagePick(t, lib.ID, "Radiohead", testMBID, "https://evil.example/x.jpg")
	w := postArtistImage(t, r, body, ct)
	var eb struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &eb)
	if w.Code != http.StatusBadRequest || !strings.Contains(eb.Error, "candidate") {
		t.Errorf("want 400 with a candidate error, got %d %q", w.Code, eb.Error)
	}
}

// TestSetArtistImage_OnlinePickRequiresFetcher: without a provider configured, an
// online pick is unavailable (upload would still work).
func TestSetArtistImage_OnlinePickRequiresFetcher(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil) // no fetcher

	body, ct := buildArtistImagePick(t, lib.ID, "Radiohead", testMBID, "https://provider.example/a.jpg")
	w := postArtistImage(t, r, body, ct)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: %s", w.Code, w.Body.String())
	}
}

// TestSetArtistImage_RescansRepresentativeTrack: the write re-indexes one track
// under the folder so the scanner re-detects the image.
func TestSetArtistImage_RescansRepresentativeTrack(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	rs := &fakeRescanner{}
	_, r, lib := newArtistImageHandler(t, root, taggedReader{"Radiohead"}, nil, rs)

	body, ct := buildArtistImageForm(t, lib.ID, "Radiohead", "x.png", pngBytes)
	if w := postArtistImage(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(rs.calls) != 1 || len(rs.calls[0]) != 1 {
		t.Fatalf("rescan calls = %v, want one call with one path", rs.calls)
	}
	want := filepath.Join(root, "Radiohead", "OK Computer", "a.flac")
	if rs.calls[0][0] != want {
		t.Errorf("rescan path = %q, want %q", rs.calls[0][0], want)
	}
}

// ----- candidate info (probe before saving) -----

func fetchArtistImageCandidateInfo(t *testing.T, r http.Handler, mbid, imgURL string) (*httptest.ResponseRecorder, *imageMetaBody) {
	t.Helper()
	reqURL := "/metadata/artist-image/candidate-info?mbid=" + url.QueryEscape(mbid) + "&url=" + url.QueryEscape(imgURL)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", reqURL, nil))
	var m *imageMetaBody
	if w.Code == http.StatusOK {
		m = &imageMetaBody{}
		if err := json.Unmarshal(w.Body.Bytes(), m); err != nil {
			t.Fatalf("decode body: %v (%s)", err, w.Body.String())
		}
	}
	return w, m
}

// TestArtistImageCandidateInfo_ReturnsMeta: probing a candidate downloads the
// real image (via the same SSRF-guarded path as the write) and reports its
// size, dimensions and format.
func TestArtistImageCandidateInfo_ReturnsMeta(t *testing.T) {
	const imgURL = "https://provider.example/full.jpg"
	fetcher := stubArtistFetcher{
		candidates: []artistimage.ImageCandidate{{FullURL: imgURL, Provider: "fanart"}},
		data:       pngBytes,
		ext:        "jpg",
	}
	_, r, _ := newArtistImageHandler(t, t.TempDir(), nullReader{}, fetcher, nil)

	w, m := fetchArtistImageCandidateInfo(t, r, testMBID, imgURL)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if m == nil || m.Width != 1 || m.Height != 1 || m.Format != "png" || m.Bytes != int64(len(pngBytes)) {
		t.Errorf("meta = %+v, want 1x1 png %d bytes", m, len(pngBytes))
	}
}

// TestArtistImageCandidateInfo_RejectsUrlNotInCandidates: the SSRF guard refuses
// a URL the provider did not offer for this MBID.
func TestArtistImageCandidateInfo_RejectsUrlNotInCandidates(t *testing.T) {
	fetcher := stubArtistFetcher{
		candidates: []artistimage.ImageCandidate{{FullURL: "https://provider.example/a.jpg", Provider: "fanart"}},
		data:       pngBytes,
	}
	_, r, _ := newArtistImageHandler(t, t.TempDir(), nullReader{}, fetcher, nil)

	w, _ := fetchArtistImageCandidateInfo(t, r, testMBID, "https://evil.example/x.jpg")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

// TestArtistImageCandidateInfo_RequiresFetcher: with no provider configured, a
// probe is unavailable.
func TestArtistImageCandidateInfo_RequiresFetcher(t *testing.T) {
	_, r, _ := newArtistImageHandler(t, t.TempDir(), nullReader{}, nil, nil)

	w, _ := fetchArtistImageCandidateInfo(t, r, testMBID, "https://provider.example/a.jpg")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: %s", w.Code, w.Body.String())
	}
}

// TestArtistImageCandidateInfo_RequiresParams: mbid and url are both required.
func TestArtistImageCandidateInfo_RequiresParams(t *testing.T) {
	_, r, _ := newArtistImageHandler(t, t.TempDir(), nullReader{}, stubArtistFetcher{}, nil)

	w, _ := fetchArtistImageCandidateInfo(t, r, "", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

// TestArtistImageServe_ReturnsFileBytes: GET serves the folder's current image.
func TestArtistImageServe_ReturnsFileBytes(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")
	if err := os.WriteFile(filepath.Join(root, "Radiohead", "artist.jpg"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil)

	w := reqArtistImage(t, r, "GET", libIDStr(lib), "Radiohead")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), pngBytes) {
		t.Errorf("served bytes do not match the file")
	}
}

// TestArtistImageServe_NotFoundWhenNoImage: no image is a 404.
func TestArtistImageServe_NotFoundWhenNoImage(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil)

	w := reqArtistImage(t, r, "GET", libIDStr(lib), "Radiohead")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
}

// TestArtistImageDelete_RemovesFile: DELETE removes the folder's current image.
func TestArtistImageDelete_RemovesFile(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")
	imgPath := filepath.Join(root, "Radiohead", "artist.jpg")
	if err := os.WriteFile(imgPath, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil)

	w := reqArtistImage(t, r, "DELETE", libIDStr(lib), "Radiohead")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(imgPath); !os.IsNotExist(err) {
		t.Errorf("artist.jpg still present after delete")
	}
}

// TestArtistImageDelete_NotFoundWhenNoImage: nothing to remove is a 404.
func TestArtistImageDelete_NotFoundWhenNoImage(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	_, r, lib := newArtistImageHandler(t, root, nullReader{}, nil, nil)

	w := reqArtistImage(t, r, "DELETE", libIDStr(lib), "Radiohead")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
}
