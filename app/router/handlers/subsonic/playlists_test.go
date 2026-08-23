package subsonic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"gorm.io/gorm"
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

// songId is a typed parameter: only track ids belong in a playlist. Ids of other
// kinds decode to a bare number, so accepting them would add the TRACK that
// happens to share that numeric id.
func TestCreatePlaylistIgnoresNonTrackSongIds(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 2)
	other, _ := s.CreatePlaylist("Other", "admin", false, nil)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Set("name", "Mix")
	q.Add("songId", encodeTrackID(tracks[0]))
	q.Add("songId", encodePlaylistID(other.ID))
	q.Add("songId", encodeAlbumID(1))
	env := getJSON(t, srv.URL, "/rest/createPlaylist.view?"+q.Encode())
	if env.SubsonicResponse.Status != "ok" {
		t.Fatalf("status=%s err=%+v", env.SubsonicResponse.Status, env.SubsonicResponse.Error)
	}
	got := env.SubsonicResponse.Playlist
	if got.SongCount != 1 || len(got.Entry) != 1 {
		t.Fatalf("expected only the track id kept, got %+v", got)
	}
	if got.Entry[0].ID != encodeTrackID(tracks[0]) {
		t.Fatalf("entry = %q, want %q", got.Entry[0].ID, encodeTrackID(tracks[0]))
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

// countSelects registers a query callback that tallies every SELECT issued
// against s, so a test can assert a handler's query count does not grow with the
// number of rows it renders.
func countSelects(t *testing.T, s *store.Store) func() int {
	t.Helper()
	var n int
	// Both callbacks: Find/First run through gorm:query, while Count and Scan
	// (which the per-row aggregate helpers use) run through gorm:row.
	if err := s.DB().Callback().Query().After("gorm:query").
		Register("test:count_selects", func(*gorm.DB) { n++ }); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	if err := s.DB().Callback().Row().After("gorm:row").
		Register("test:count_rows", func(*gorm.DB) { n++ }); err != nil {
		t.Fatalf("register row callback: %v", err)
	}
	return func() int { return n }
}

// getPlaylists must issue a fixed number of queries regardless of how many
// playlists it returns: the per-row count and duration lookups are the N+1 this
// pins shut. Stars and play stats above them are already batched.
func TestGetPlaylistsDoesNotScaleQueriesWithRowCount(t *testing.T) {
	s := testStore(t)
	tracks := seedTracks(t, s, 2)
	for i := range 6 {
		if _, err := s.CreatePlaylist(fmt.Sprintf("PL %d", i), "admin", false, tracks); err != nil {
			t.Fatal(err)
		}
	}

	srv := newTestServer(t, s)
	defer srv.Close()

	selects := countSelects(t, s)
	env := getJSON(t, srv.URL, "/rest/getPlaylists.view")
	if got := len(env.SubsonicResponse.Playlists.Playlist); got != 6 {
		t.Fatalf("expected 6 playlists, got %d", got)
	}
	// Batched: the playlist list, the star lookup, the play stats and the track
	// aggregate — a small constant. Per-row lookups would add 2 per playlist.
	if n := selects(); n > 6 {
		t.Errorf("getPlaylists issued %d SELECTs for 6 playlists; expected a constant handful (per-row count/duration queries are the N+1)", n)
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

	key := assetkey.PlaylistOf(pl)
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

	// getCoverArt must serve a derivative of the uploaded image, not the
	// generated fallback.
	if !servesUploadedCover(t, srv.URL, encodePlaylistID(pl.ID)) {
		t.Fatal("getCoverArt should serve the uploaded cover")
	}
}

func TestUpdatePlaylistMultipartCoverClear(t *testing.T) {
	s := testStore(t)
	pl, _ := s.CreatePlaylist("Mix", "admin", false, nil)
	srv, as := newRadioServer(t, s)
	defer srv.Close()

	key := assetkey.PlaylistOf(pl)
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

	key := assetkey.PlaylistOf(pl)
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

	stats, err := s.PlaylistStats("admin", []uint{pl.ID})
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
	if err := s.Star("admin", "playlist", starredPl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlaylistPlay("admin", starredPl.ID, time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)); err != nil {
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
	if err := s.Star("admin", "playlist", pl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPlaylistPlay("admin", pl.ID, time.Now()); err != nil {
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

// TestScrobbleAbsurdTimeBounded verifies that a scrobble request with an absurd
// `time` parameter (far future or pre-epoch) does not break getPlaylists.
func TestScrobbleAbsurdTimeBounded(t *testing.T) {
	s := testStore(t)
	db := s.DB()
	pl := model.Playlist{Name: "ScrobbleTest", Owner: "admin"}
	db.Create(&pl)

	srv := newTestServer(t, s)
	defer srv.Close()

	// Scrobble with a far-future epoch-ms timestamp (year 10000).
	// The handler must accept it and bound it to a sane range.
	farFuture := "253402300800000"
	scrobbleEnv := getJSON(t, srv.URL, fmt.Sprintf("/rest/scrobble.view?id=pl-%d&time=%s", pl.ID, farFuture))
	if scrobbleEnv.SubsonicResponse.Status != "ok" {
		t.Fatalf("scrobble failed: %+v", scrobbleEnv.SubsonicResponse.Error)
	}

	// getPlaylists must remain working even if the stored timestamp is malformed.
	listEnv := getJSON(t, srv.URL, "/rest/getPlaylists.view")
	if listEnv.SubsonicResponse.Status != "ok" {
		t.Fatalf("getPlaylists failed after absurd scrobble: %+v", listEnv.SubsonicResponse.Error)
	}
	if len(listEnv.SubsonicResponse.Playlists.Playlist) != 1 {
		t.Fatalf("expected 1 playlist, got %d", len(listEnv.SubsonicResponse.Playlists.Playlist))
	}
}

func TestPlaylistVisibilityAndWriteGuards(t *testing.T) {
	s := testStore(t)
	pl, err := s.CreatePlaylist("demo private", "demo", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := s.CreatePlaylist("demo public", "demo", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	do := func(user, path string) errorEnvelope {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		req.Header.Set("X-Test-User", user)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body errorEnvelope
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	plID := func(id uint) string { return "pl-" + fmt.Sprintf("%d", id) }

	// admin cannot read demo's private playlist: 70, not 50 (no existence leak).
	if b := do("admin", "/rest/getPlaylist?id="+plID(pl.ID)); b.SubsonicResponse.Error == nil || b.SubsonicResponse.Error.Code != 70 {
		t.Fatalf("expected 70 reading foreign private playlist, got %+v", b.SubsonicResponse.Error)
	}
	// admin can read demo's public playlist.
	if b := do("admin", "/rest/getPlaylist?id="+plID(pub.ID)); b.SubsonicResponse.Status != "ok" {
		t.Fatalf("expected ok reading public playlist, got %+v", b.SubsonicResponse)
	}
	// admin cannot delete demo's public playlist: 50 (visible but not owned).
	if b := do("admin", "/rest/deletePlaylist?id="+plID(pub.ID)); b.SubsonicResponse.Error == nil || b.SubsonicResponse.Error.Code != 50 {
		t.Fatalf("expected 50 deleting foreign playlist, got %+v", b.SubsonicResponse.Error)
	}
	// the owner can delete it.
	if b := do("demo", "/rest/deletePlaylist?id="+plID(pub.ID)); b.SubsonicResponse.Status != "ok" {
		t.Fatalf("owner delete failed: %+v", b.SubsonicResponse)
	}
}
