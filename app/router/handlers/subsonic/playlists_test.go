package subsonic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

type playlistEntry struct {
	ID string `json:"id"`
}

type playlistObj struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Comment   string          `json:"comment"`
	Owner     string          `json:"owner"`
	Public    bool            `json:"public"`
	SongCount int             `json:"songCount"`
	Duration  int             `json:"duration"`
	Starred   string          `json:"starred"`
	PlayCount int             `json:"playCount"`
	Played    string          `json:"played"`
	Entry     []playlistEntry `json:"entry"`
}

type playlistEnvelope struct {
	SubsonicResponse struct {
		Status    string `json:"status"`
		Playlists struct {
			Playlist []playlistObj `json:"playlist"`
		} `json:"playlists"`
		Playlist playlistObj `json:"playlist"`
		Error    struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func decodePlaylist(t *testing.T, resp *http.Response) playlistEnvelope {
	t.Helper()
	var body playlistEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

// seedTracks inserts n tracks under one album and returns their track IDs.
func seedTracks(t *testing.T, s *store.Store, n int) []uint {
	t.Helper()
	db := s.DB()
	album := model.Album{Name: "A", NameNorm: "a", AlbumArtistNorm: "x"}
	db.Create(&album)
	ids := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		tr := model.Track{
			AlbumID:  album.ID,
			Filename: fmt.Sprintf("%02d.mp3", i+1),
			FilePath: fmt.Sprintf("/%02d.mp3", i+1),
			Duration: 100 * (i + 1),
		}
		db.Create(&tr)
		ids = append(ids, tr.ID)
	}
	return ids
}

func getJSON(t *testing.T, srvURL, path string) playlistEnvelope {
	t.Helper()
	resp, err := http.Get(srvURL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	return decodePlaylist(t, resp)
}

func TestCreatePlaylistHandler(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 2)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Set("name", "My Mix")
	q.Add("songId", encodeTrackID(tracks[0]))
	q.Add("songId", encodeTrackID(tracks[1]))
	env := getJSON(t, srv.URL, "/rest/createPlaylist.view?"+q.Encode())
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	pl := env.SubsonicResponse.Playlist
	if len(pl.ID) < 3 || pl.ID[:3] != "pl-" {
		t.Fatalf("expected pl- prefix, got %q", pl.ID)
	}
	if pl.Name != "My Mix" || pl.SongCount != 2 || len(pl.Entry) != 2 {
		t.Fatalf("unexpected full object: %+v", pl)
	}
	if pl.Owner != "admin" {
		t.Errorf("expected owner admin, got %q", pl.Owner)
	}
	if pl.Duration != 300 {
		t.Errorf("expected duration 300, got %d", pl.Duration)
	}
}

func TestCreatePlaylistMissingName(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/createPlaylist.view")
	if env.SubsonicResponse.Status != "failed" || env.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected failed + code 10, got %+v", env.SubsonicResponse)
	}
}

func TestCreatePlaylistPublic(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/createPlaylist.view?name=Pub&public=true")
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	if !env.SubsonicResponse.Playlist.Public {
		t.Fatal("expected public=true in response")
	}
	var loaded model.Playlist
	s.DB().First(&loaded)
	if !loaded.Public {
		t.Fatal("expected public persisted")
	}
}

func TestCreatePlaylistWithPlaylistIdReplacesTracks(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 3)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, []uint{tracks[0], tracks[1]})
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Set("playlistId", encodePlaylistID(pl.ID))
	q.Add("songId", encodeTrackID(tracks[2]))
	env := getJSON(t, srv.URL, "/rest/createPlaylist.view?"+q.Encode())
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	got := env.SubsonicResponse.Playlist
	if got.SongCount != 1 || len(got.Entry) != 1 || got.Entry[0].ID != encodeTrackID(tracks[2]) {
		t.Fatalf("expected only tr-%d after replace, got %+v", tracks[2], got)
	}
}

func TestCreatePlaylistWithPlaylistIdNotFound(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/createPlaylist.view?playlistId=pl-9999")
	if env.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected code 70, got %+v", env.SubsonicResponse.Error)
	}
}

func TestGetPlaylistsHandler(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 2)
	_, _ = s.CreatePlaylist("Bravo", "admin", false, []uint{tracks[0], tracks[1]})
	_, _ = s.CreatePlaylist("Alpha", "admin", false, nil)
	srv := newTestServer(t, s)
	defer srv.Close()

	env := getJSON(t, srv.URL, "/rest/getPlaylists.view")
	pls := env.SubsonicResponse.Playlists.Playlist
	if len(pls) != 2 {
		t.Fatalf("expected 2, got %d", len(pls))
	}
	// Sorted by name ASC.
	if pls[0].Name != "Alpha" || pls[1].Name != "Bravo" {
		t.Fatalf("unexpected order: %+v", pls)
	}
	if pls[1].SongCount != 2 || pls[1].Duration != 300 {
		t.Fatalf("expected songCount/duration on Bravo, got %+v", pls[1])
	}
}

func TestGetPlaylistHandler(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 2)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, []uint{tracks[0], tracks[1]})
	srv := newTestServer(t, s)
	defer srv.Close()

	env := getJSON(t, srv.URL, "/rest/getPlaylist.view?id="+encodePlaylistID(pl.ID))
	got := env.SubsonicResponse.Playlist
	if got.SongCount != 2 || len(got.Entry) != 2 || got.Duration != 300 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestGetPlaylistMissingID(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/getPlaylist.view")
	if env.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected code 10, got %+v", env.SubsonicResponse.Error)
	}
}

func TestGetPlaylistNotFound(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/getPlaylist.view?id=pl-9999")
	if env.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected code 70, got %+v", env.SubsonicResponse.Error)
	}
}

func TestUpdatePlaylistRenameAndPublic(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Old", "admin", false, nil)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Set("playlistId", encodePlaylistID(pl.ID))
	q.Set("name", "New")
	q.Set("comment", "notes")
	q.Set("public", "true")
	env := getJSON(t, srv.URL, "/rest/updatePlaylist.view?"+q.Encode())
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	var loaded model.Playlist
	s.DB().First(&loaded, pl.ID)
	if loaded.Name != "New" || loaded.Comment != "notes" || !loaded.Public {
		t.Fatalf("not updated: %+v", loaded)
	}
}

func TestUpdatePlaylistAddAndRemoveTracks(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 3)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, []uint{tracks[0], tracks[1]})
	srv := newTestServer(t, s)
	defer srv.Close()

	// Add track 3, remove index 0 (track 1).
	q := url.Values{}
	q.Set("playlistId", encodePlaylistID(pl.ID))
	q.Add("songIdToAdd", encodeTrackID(tracks[2]))
	q.Set("songIndexToRemove", "0")
	env := getJSON(t, srv.URL, "/rest/updatePlaylist.view?"+q.Encode())
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	got := getJSON(t, srv.URL, "/rest/getPlaylist.view?id="+encodePlaylistID(pl.ID)).SubsonicResponse.Playlist
	if got.SongCount != 2 {
		t.Fatalf("expected 2 tracks, got %d", got.SongCount)
	}
	// Order preserved: track 2 (index 1 originally) then the added track 3.
	if got.Entry[0].ID != encodeTrackID(tracks[1]) || got.Entry[1].ID != encodeTrackID(tracks[2]) {
		t.Fatalf("unexpected order after add/remove: %+v", got.Entry)
	}
}

func TestUpdatePlaylistMissingID(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/updatePlaylist.view?name=X")
	if env.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected code 10, got %+v", env.SubsonicResponse.Error)
	}
}

func TestDeletePlaylistHandler(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Temp", "admin", false, nil)
	srv := newTestServer(t, s)
	defer srv.Close()

	env := getJSON(t, srv.URL, "/rest/deletePlaylist.view?id="+encodePlaylistID(pl.ID))
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	var count int64
	s.DB().Model(&model.Playlist{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestUpdatePlaylistMultipartWithCover(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, nil)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	key := playlistCoverKey(pl.ID)
	if _, ok := as.Get(assetstore.KindPlaylist, key); ok {
		t.Fatal("cover should not exist before upload")
	}

	body, contentType := buildMultipart(t, map[string]string{
		"playlistId": encodePlaylistID(pl.ID),
	}, pngBytes(t), "c.png")
	resp, err := http.Post(srv.URL+"/rest/updatePlaylist.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	env := decodePlaylist(t, resp)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	if _, ok := as.Get(assetstore.KindPlaylist, key); !ok {
		t.Fatal("expected cover in asset store after upload")
	}

	// getCoverArt must serve the uploaded image (an actual PNG), not a
	// generated fallback.
	cov, err := http.Get(srv.URL + "/rest/getCoverArt.view?id=" + encodePlaylistID(pl.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cov.Body.Close() }()
	if cov.StatusCode != http.StatusOK {
		t.Fatalf("getCoverArt status=%d", cov.StatusCode)
	}
	data, _ := io.ReadAll(cov.Body)
	if detectImageContentType(data) != "image/png" {
		t.Fatalf("expected served png, got content-type %q", detectImageContentType(data))
	}
}

func TestUpdatePlaylistMultipartCoverClear(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, nil)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	key := playlistCoverKey(pl.ID)
	if err := as.PutManual(assetstore.KindPlaylist, key, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}

	body, contentType := buildMultipart(t, map[string]string{
		"playlistId": encodePlaylistID(pl.ID),
		"coverClear": "true",
	}, nil, "")
	resp, err := http.Post(srv.URL+"/rest/updatePlaylist.view", contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	env := decodePlaylist(t, resp)
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	if _, ok := as.Get(assetstore.KindPlaylist, key); ok {
		t.Fatal("expected cover removed after coverClear")
	}
}

func TestDeletePlaylistRemovesCover(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Temp", "admin", false, nil)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	key := playlistCoverKey(pl.ID)
	if err := as.PutManual(assetstore.KindPlaylist, key, "png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}

	env := getJSON(t, srv.URL, "/rest/deletePlaylist.view?id="+encodePlaylistID(pl.ID))
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	if _, ok := as.Get(assetstore.KindPlaylist, key); ok {
		t.Fatal("expected cover removed after playlist delete")
	}
}

func TestDeletePlaylistMissingID(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/deletePlaylist.view")
	if env.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected code 10, got %+v", env.SubsonicResponse.Error)
	}
}

func TestDeletePlaylistNotFound(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()
	env := getJSON(t, srv.URL, "/rest/deletePlaylist.view?id=pl-9999")
	if env.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected code 70, got %+v", env.SubsonicResponse.Error)
	}
}

func TestScrobblePlaylistRecordsPlay(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 1)
	pl, err := s.CreatePlaylist("Mix", "admin", true, tracks)
	if err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/scrobble.view?id=pl-%d", srv.URL, pl.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	stats, err := s.PlaylistStats([]uint{pl.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := stats[pl.ID].PlayCount; got != 1 {
		t.Fatalf("PlayCount = %d, want 1", got)
	}
}

func TestScrobbleTrackStillRecordsTrackPlay(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 1)
	srv := newTestServer(t, s)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s/rest/scrobble.view?id=tr-%d", srv.URL, tracks[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	var n int64
	s.DB().Model(&model.PlayHistory{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected 1 track play, got %d", n)
	}
}

func TestGetPlaylistsIncludesStarAndPlayFields(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 2)
	starredPl, err := s.CreatePlaylist("Fav", "admin", true, tracks)
	if err != nil {
		t.Fatal(err)
	}
	plainPl, err := s.CreatePlaylist("Plain", "admin", true, tracks)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Star("playlist", starredPl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlaylistPlay(starredPl.ID, time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	body := getJSON(t, srv.URL, "/rest/getPlaylists.view")
	byName := map[string]playlistObj{}
	for _, pl := range body.SubsonicResponse.Playlists.Playlist {
		byName[pl.Name] = pl
	}

	fav := byName["Fav"]
	if fav.Starred == "" {
		t.Fatal("starred playlist must carry a starred timestamp")
	}
	if fav.PlayCount != 1 {
		t.Fatalf("Fav playCount = %d, want 1", fav.PlayCount)
	}
	if fav.Played == "" {
		t.Fatal("played playlist must carry a played timestamp")
	}

	plain := byName["Plain"]
	if plain.Starred != "" {
		t.Fatalf("unstarred playlist must omit starred, got %q", plain.Starred)
	}
	if plain.PlayCount != 0 {
		t.Fatalf("Plain playCount = %d, want 0", plain.PlayCount)
	}
	if plain.Played != "" {
		t.Fatalf("never-played playlist must omit played, got %q", plain.Played)
	}
	_ = plainPl
}

func TestGetPlaylistIncludesStarAndPlayFields(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 1)
	pl, err := s.CreatePlaylist("Solo", "admin", true, tracks)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Star("playlist", pl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlaylistPlay(pl.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	body := getJSON(t, srv.URL, fmt.Sprintf("/rest/getPlaylist.view?id=pl-%d", pl.ID))
	got := body.SubsonicResponse.Playlist
	if got.Starred == "" || got.Played == "" || got.PlayCount != 1 {
		t.Fatalf("getPlaylist starred=%q played=%q playCount=%d", got.Starred, got.Played, got.PlayCount)
	}
}
