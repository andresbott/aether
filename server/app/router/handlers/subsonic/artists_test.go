package subsonic

import (
	"bytes"
	"encoding/json"
	"image"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
)

// postStatus posts a multipart body and returns the subsonic status + error code.
func postArtist(t *testing.T, srvURL string, body io.Reader, contentType string) (string, int) {
	t.Helper()
	resp, err := http.Post(srvURL+"/rest/updateArtist.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env struct {
		SubsonicResponse struct {
			Status string `json:"status"`
			Error  struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return env.SubsonicResponse.Status, env.SubsonicResponse.Error.Code
}

// servesUploadedCover reports whether getCoverArt answers with the uploaded
// image rather than the generated fallback. The served bytes are a re-encoded
// derivative, not the upload verbatim, so the check is on dimensions: uploads in
// these tests are tiny, and a generated cover is always full-size.
func servesUploadedCover(t *testing.T, srvURL, id string) bool {
	t.Helper()
	resp, err := http.Get(srvURL + "/rest/getCoverArt.view?id=" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("getCoverArt status=%d", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("served cover is not a decodable image: %v", err)
	}
	return cfg.Width == uploadedCoverEdge && cfg.Height == uploadedCoverEdge
}

// uploadedCoverEdge is the edge length of the pngBytes fixture the multipart
// upload helpers post. Sources are never upscaled, so a derivative of it keeps
// these dimensions.
const uploadedCoverEdge = 2

// largerPNG creates a 4x4 PNG, larger than uploadedCoverEdge, so tests can
// distinguish which slot was served when both are seeded.
func largerPNG(t *testing.T) []byte {
	t.Helper()
	return realPNG(t, 4, 4)
}

func TestUpdateArtistMatchedUploadsUnderMBID(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Matched", NameNorm: "matched", MBArtistID: "mbid-up"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-up"); !ok {
		t.Fatal("expected cover under MBID key")
	}
	if !servesUploadedCover(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the uploaded png")
	}
}

func TestUpdateArtistCoverClear(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Clear", NameNorm: "clear", MBArtistID: "mbid-clear"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	if err := as.PutManual(assetstore.KindArtist, "mbid-clear", "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeArtistID(artist.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}
	if _, ok := as.Get(assetstore.KindArtist, "mbid-clear"); ok {
		t.Fatal("expected cover removed after coverClear")
	}
}

// updateArtist writes global catalog data, so it must require admin privileges
// (error 50 for non-admins), matching updateAlbum and updateGenre.
func TestUpdateArtistRequiresAdmin(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Radiohead", NameNorm: "radiohead"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv := newRadioServerWithRoles(t, s, "root")
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/rest/updateArtist.view", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Test-User", "bob")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var env struct {
		SubsonicResponse struct {
			Status string `json:"status"`
			Error  struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.SubsonicResponse.Status != "failed" || env.SubsonicResponse.Error.Code != 50 {
		t.Fatalf("expected error 50 for non-admin, got status=%s code=%d", env.SubsonicResponse.Status, env.SubsonicResponse.Error.Code)
	}
}

func TestUpdateArtistRejectsNonMultipart(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/updateArtist.view?id=ar-1")
	if env.SubsonicResponse.Status != "failed" {
		t.Fatalf("expected failed for non-multipart, got %s", env.SubsonicResponse.Status)
	}
}

func TestUpdateArtistNotFound(t *testing.T) {
	s := testStore(t)
	srv, _ := newRadioServer(t, s)
	defer srv.Close()
	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(999)}, pngBytes(t), "a.png")
	if _, code := postArtist(t, srv.URL, body, ct); code != 70 {
		t.Fatalf("expected code 70, got %d", code)
	}
}

// --- Task 5: Identity-keyed artist covers ---

// An unmatched artist's upload must land under assetkey.Artist("", nameNorm),
// not under strconv(id), so a DB rebuild re-attaches it correctly.
func TestUpdateArtistUnmatchedUsesNameHashKey(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Unmatched", NameNorm: "unmatched"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}

	// Must be stored under the name-hash key, not the DB ID.
	expectedKey := assetkey.Artist("", artist.NameNorm)
	if _, ok := as.Get(assetstore.KindArtist, expectedKey); !ok {
		t.Fatalf("expected cover under name-hash key %q", expectedKey)
	}
	// Must NOT be under the old DB-ID key.
	dbKey := strconv.FormatUint(uint64(artist.ID), 10)
	if _, ok := as.Get(assetstore.KindArtist, dbKey); ok {
		t.Fatalf("cover is still under DB-ID key %q; migration incomplete", dbKey)
	}
	// Must serve back through getCoverArt.
	if !servesUploadedCover(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the uploaded png")
	}
}

// A matched artist's upload must land under the literal MBID (unchanged
// behaviour), so the auto-fetcher's slot remains durable.
func TestUpdateArtistMatchedUsesLiteralMBID(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Matched", NameNorm: "matched", MBArtistID: "mbid-literal"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}

	// Must be stored under the literal MBID.
	if _, ok := as.Get(assetstore.KindArtist, "mbid-literal"); !ok {
		t.Fatal("expected cover under literal MBID key")
	}
	if !servesUploadedCover(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the uploaded png")
	}
}

// coverClear on a matched artist must clear BOTH the MBID slot and the
// name-hash slot, so a stale unmatched-slot image cannot resurface.
func TestUpdateArtistCoverClearBothSlots(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Clear", NameNorm: "clear", MBArtistID: "mbid-clear-both"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	// Seed both slots: the MBID slot (current), and the name-hash slot (stale).
	mbidKey := "mbid-clear-both"
	nameHashKey := assetkey.Artist("", artist.NameNorm)
	if err := as.PutManual(assetstore.KindArtist, mbidKey, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	if err := as.PutManual(assetstore.KindArtist, nameHashKey, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}

	body, ct := buildMultipart(t, map[string]string{
		"id":         encodeArtistID(artist.ID),
		"coverClear": "true",
	}, nil, "")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}

	// Both slots must be cleared.
	if _, ok := as.Get(assetstore.KindArtist, mbidKey); ok {
		t.Fatal("MBID slot was not cleared")
	}
	if _, ok := as.Get(assetstore.KindArtist, nameHashKey); ok {
		t.Fatal("name-hash slot was not cleared; stale image can resurface")
	}
}

// The MBID slot must be checked before the name-hash slot, so a matched
// artist's MBID-slot image takes precedence over a stale name-hash-slot image
// (e.g., one uploaded before the artist was matched).
func TestUpdateArtistMBIDSlotBeatsNameHashSlot(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Ordered", NameNorm: "ordered", MBArtistID: "mbid-ordered"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	// Seed BOTH slots with distinguishable images. The MBID slot holds a 2x2 PNG
	// (pngBytes fixture), the name-hash slot holds a 4x4 PNG. If the read order
	// were swapped, getCoverArt would serve the 4x4.
	mbidKey := "mbid-ordered"
	nameHashKey := assetkey.Artist("", artist.NameNorm)
	if err := as.PutManual(assetstore.KindArtist, mbidKey, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	if err := as.PutManual(assetstore.KindArtist, nameHashKey, "png", largerPNG(t)); err != nil {
		t.Fatal(err)
	}

	// getCoverArt must serve the MBID-slot image (2x2), not the name-hash-slot one (4x4).
	if !servesUploadedCover(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the 2x2 MBID-slot image, not the 4x4 name-hash-slot image")
	}
}

// A manual upload must outrank an auto-fetched image within the same slot.
func TestUpdateArtistManualOutranksAutoWithinSlot(t *testing.T) {
	s := testStore(t)
	artist := model.Artist{Name: "Ranked", NameNorm: "ranked", MBArtistID: "mbid-ranked"}
	if err := s.DB().Create(&artist).Error; err != nil {
		t.Fatalf("create artist: %v", err)
	}
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	// Seed the MBID slot with an auto-fetched image.
	if err := as.PutAuto(assetstore.KindArtist, "mbid-ranked", "png", []byte("auto")); err != nil {
		t.Fatal(err)
	}

	// Upload a manual image into the same slot.
	body, ct := buildMultipart(t, map[string]string{"id": encodeArtistID(artist.ID)}, pngBytes(t), "a.png")
	if status, code := postArtist(t, srv.URL, body, ct); status != "ok" {
		t.Fatalf("status=%s code=%d", status, code)
	}

	// The manual image must outrank the auto-fetched one in the MBID slot.
	p, manual, ok := as.GetEntry(assetstore.KindArtist, "mbid-ranked")
	if !ok {
		t.Fatal("no image in MBID slot after upload")
	}
	if !manual {
		t.Fatalf("MBID slot still holds auto-fetched image (%s); manual upload did not outrank auto", p)
	}
	if !servesUploadedCover(t, srv.URL, encodeArtistID(artist.ID)) {
		t.Fatal("getCoverArt should serve the manual upload")
	}
}
