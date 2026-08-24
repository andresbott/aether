package metadata_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/artistimage"
	"github.com/andresbott/aether/internal/coverart"
)

// countingCoverArt is a CoverArtClient double that counts image downloads, so a
// test can assert that a repeated probe and the save reuse one download.
type countingCoverArt struct {
	data      []byte
	ext       string
	downloads int
}

func (c *countingCoverArt) List(context.Context, string, string) ([]coverart.CoverImage, error) {
	return nil, nil
}
func (c *countingCoverArt) DownloadImage(context.Context, string) ([]byte, string, error) {
	c.downloads++
	return c.data, c.ext, nil
}

// countingArtistFetcher counts both List and Download, so a test can assert a
// cache hit skips the SSRF re-list as well as the download.
type countingArtistFetcher struct {
	candidates []artistimage.ImageCandidate
	data       []byte
	ext        string
	lists      int
	downloads  int
}

func (c *countingArtistFetcher) List(context.Context, string) ([]artistimage.ImageCandidate, error) {
	c.lists++
	return c.candidates, nil
}
func (c *countingArtistFetcher) Download(context.Context, string, string) ([]byte, string, error) {
	c.downloads++
	return c.data, c.ext, nil
}

// buildPictureImageURLForm builds a POST /metadata/pictures body that saves from
// a provider URL ("image_url") rather than an uploaded file.
func buildPictureImageURLForm(
	t *testing.T, libID uint, target, pictureType string, paths []string, imgURL string,
) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("library_id", strconv.FormatUint(uint64(libID), 10))
	_ = mw.WriteField("slot", target)
	if pictureType != "" {
		_ = mw.WriteField("type", pictureType)
	}
	for _, p := range paths {
		_ = mw.WriteField("paths", p)
	}
	_ = mw.WriteField("image_url", imgURL)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

// TestPictureProbeThenSave_DownloadsOnce: probing an album cover candidate twice
// and then saving it downloads the image exactly once.
func TestPictureProbeThenSave_DownloadsOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "album"), 0o755); err != nil {
		t.Fatal(err)
	}
	const imgURL = "https://coverart.example/full.png"
	ca := &countingCoverArt{data: pngBytes, ext: "png"}
	_, r, lib := newPictureHandler(t, root, ca)

	if w, _ := fetchPictureCandidateInfo(t, r, imgURL); w.Code != http.StatusOK {
		t.Fatalf("probe 1 status %d", w.Code)
	}
	if w, _ := fetchPictureCandidateInfo(t, r, imgURL); w.Code != http.StatusOK {
		t.Fatalf("probe 2 status %d", w.Code)
	}
	body, ct := buildPictureImageURLForm(t, lib.ID, "folder", "Back Cover", []string{"album/01.flac"}, imgURL)
	if w := postPicture(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("save status %d: %s", w.Code, w.Body.String())
	}

	if ca.downloads != 1 {
		t.Fatalf("DownloadImage called %d times, want 1 (two probes + save should reuse one download)", ca.downloads)
	}
}

// TestArtistProbeThenSave_DownloadsAndListsOnce: probing an artist portrait
// candidate twice and then saving it downloads once and re-lists once (the
// cache hit skips both the download and the SSRF re-list).
func TestArtistProbeThenSave_DownloadsAndListsOnce(t *testing.T) {
	root := t.TempDir()
	mkAlbumTrack(t, root, "Radiohead", "OK Computer")

	const imgURL = "https://provider.example/full.jpg"
	fetcher := &countingArtistFetcher{
		candidates: []artistimage.ImageCandidate{{FullURL: imgURL, Provider: "fanart"}},
		data:       pngBytes,
		ext:        "jpg",
	}
	_, r, lib := newArtistImageHandler(t, root, nullReader{}, fetcher, nil)

	probe := func() int {
		reqURL := "/metadata/artist-image/candidate-info?mbid=" +
			url.QueryEscape(testMBID) + "&url=" + url.QueryEscape(imgURL)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", reqURL, nil))
		return w.Code
	}
	if code := probe(); code != http.StatusOK {
		t.Fatalf("probe 1 status %d", code)
	}
	if code := probe(); code != http.StatusOK {
		t.Fatalf("probe 2 status %d", code)
	}
	body, ct := buildArtistImagePick(t, lib.ID, "Radiohead", testMBID, imgURL)
	if w := postArtistImage(t, r, body, ct); w.Code != http.StatusOK {
		t.Fatalf("save status %d: %s", w.Code, w.Body.String())
	}

	if fetcher.downloads != 1 {
		t.Fatalf("Download called %d times, want 1 (two probes + save should reuse one download)", fetcher.downloads)
	}
	if fetcher.lists != 1 {
		t.Fatalf("List called %d times, want 1 (a cache hit should skip the SSRF re-list)", fetcher.lists)
	}
}
