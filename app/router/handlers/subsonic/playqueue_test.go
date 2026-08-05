package subsonic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

// playQueueBody covers both the id-based and index-based response shapes, so one
// decode target serves every assertion in this file.
type playQueueBody struct {
	SubsonicResponse struct {
		Status    string `json:"status"`
		PlayQueue *struct {
			Current   string `json:"current"`
			Position  int64  `json:"position"`
			Username  string `json:"username"`
			Changed   string `json:"changed"`
			ChangedBy string `json:"changedBy"`
			Entry     []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"entry"`
		} `json:"playQueue"`
		PlayQueueByIndex *struct {
			CurrentIndex int    `json:"currentIndex"`
			Position     int64  `json:"position"`
			Username     string `json:"username"`
			Changed      string `json:"changed"`
			ChangedBy    string `json:"changedBy"`
			Entry        []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"entry"`
		} `json:"playQueueByIndex"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func getPlayQueueJSON(t *testing.T, srv *httptest.Server, path string) playQueueBody {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body playQueueBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

// seedQueueTracks creates three tracks with titles, returned in queue order.
func seedQueueTracks(t *testing.T, s *store.Store) []model.Track {
	t.Helper()
	db := s.DB()
	album := model.Album{Name: "Alb", NameNorm: "alb", AlbumArtistNorm: "a"}
	db.Create(&album)
	tracks := make([]model.Track, 0, 3)
	for _, title := range []string{"One", "Two", "Three"} {
		tr := model.Track{AlbumID: album.ID, Filename: title + ".mp3", FilePath: "/" + title + ".mp3", Title: title, Duration: 600}
		db.Create(&tr)
		tracks = append(tracks, tr)
	}
	return tracks
}

func TestSavePlayQueueThenGetPlayQueue(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Add("id", encodeTrackID(tracks[1].ID))
	q.Add("id", encodeTrackID(tracks[2].ID))
	q.Set("current", encodeTrackID(tracks[1].ID))
	q.Set("position", "42000")
	q.Set("c", "test-client")

	body := getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode())
	if body.SubsonicResponse.Status != "ok" {
		t.Fatalf("savePlayQueue status = %q, error = %+v", body.SubsonicResponse.Status, body.SubsonicResponse.Error)
	}

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueue.view")
	pq := got.SubsonicResponse.PlayQueue
	if pq == nil {
		t.Fatal("expected a playQueue element")
	}
	if len(pq.Entry) != 3 {
		t.Fatalf("len(entry) = %d, want 3", len(pq.Entry))
	}
	if pq.Entry[0].Title != "One" || pq.Entry[2].Title != "Three" {
		t.Fatalf("queue order lost: %+v", pq.Entry)
	}
	if want := encodeTrackID(tracks[1].ID); pq.Current != want {
		t.Fatalf("current = %q, want %q", pq.Current, want)
	}
	if pq.Position != 42000 {
		t.Fatalf("position = %d, want 42000", pq.Position)
	}
	if pq.ChangedBy != "test-client" {
		t.Fatalf("changedBy = %q, want test-client", pq.ChangedBy)
	}
	if pq.Changed == "" {
		t.Fatal("expected a changed timestamp")
	}
	if pq.Username == "" {
		t.Fatal("expected a username on the saved queue")
	}
}

// The spec's default: a save with no position means the current track starts from
// the beginning.
func TestSavePlayQueueDefaultsPositionToZero(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Set("current", encodeTrackID(tracks[0].ID))
	if body := getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode()); body.SubsonicResponse.Status != "ok" {
		t.Fatalf("save failed: %+v", body.SubsonicResponse.Error)
	}

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueue.view")
	if got.SubsonicResponse.PlayQueue.Position != 0 {
		t.Fatalf("position = %d, want 0", got.SubsonicResponse.PlayQueue.Position)
	}
}

// "Send a call without any parameters to clear the currently saved queue."
func TestSavePlayQueueWithoutIdsClearsTheQueue(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Set("current", encodeTrackID(tracks[0].ID))
	_ = getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode())

	if body := getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view"); body.SubsonicResponse.Status != "ok" {
		t.Fatalf("clearing save failed: %+v", body.SubsonicResponse.Error)
	}

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueue.view")
	if got.SubsonicResponse.Status != "ok" {
		t.Fatalf("getPlayQueue after clear should still succeed, got %+v", got.SubsonicResponse.Error)
	}
	if got.SubsonicResponse.PlayQueue != nil {
		t.Fatalf("expected no playQueue element after a clear, got %+v", got.SubsonicResponse.PlayQueue)
	}
}

// An unsaved queue is not an error — a fresh client asks before it ever saves.
func TestGetPlayQueueWithoutSavedQueue(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueue.view")
	if got.SubsonicResponse.Status != "ok" {
		t.Fatalf("status = %q, want ok", got.SubsonicResponse.Status)
	}
	if got.SubsonicResponse.PlayQueue != nil {
		t.Fatal("expected no playQueue element when nothing was ever saved")
	}
}

// current names a track id; with duplicates in the queue it is inherently
// ambiguous, so it resolves to the FIRST matching slot. Clients that care use the
// index-based variant.
func TestSavePlayQueueResolvesCurrentToFirstMatchingSlot(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Add("id", encodeTrackID(tracks[1].ID))
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Set("current", encodeTrackID(tracks[0].ID))
	q.Set("position", "1000")
	_ = getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode())

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueueByIndex.view")
	pq := got.SubsonicResponse.PlayQueueByIndex
	if pq == nil {
		t.Fatal("expected a playQueueByIndex element")
	}
	if pq.CurrentIndex != 0 {
		t.Fatalf("currentIndex = %d, want 0 (first match)", pq.CurrentIndex)
	}
}

// A current id that is not in the queue at all is a client error: the spec
// requires current to be one of the supplied ids.
func TestSavePlayQueueRejectsCurrentNotInQueue(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Set("current", encodeTrackID(tracks[2].ID))
	body := getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode())
	if body.SubsonicResponse.Status != "failed" {
		t.Fatal("expected a failed response when current is absent from the queue")
	}
	if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected error code 10, got %+v", body.SubsonicResponse.Error)
	}
}

// current is required as soon as ids are present (OpenSubsonic: "required unless
// id is empty").
func TestSavePlayQueueRequiresCurrentWhenIdsPresent(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	body := getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode())
	if body.SubsonicResponse.Status != "failed" {
		t.Fatal("expected a failed response when current is missing but ids were sent")
	}
	if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected error code 10, got %+v", body.SubsonicResponse.Error)
	}
}

func TestSavePlayQueueByIndexThenGetByIndex(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Add("id", encodeTrackID(tracks[1].ID))
	q.Set("currentIndex", "1")
	q.Set("position", "77000")
	q.Set("c", "idx-client")
	if body := getPlayQueueJSON(t, srv, "/rest/savePlayQueueByIndex.view?"+q.Encode()); body.SubsonicResponse.Status != "ok" {
		t.Fatalf("save failed: %+v", body.SubsonicResponse.Error)
	}

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueueByIndex.view")
	pq := got.SubsonicResponse.PlayQueueByIndex
	if pq == nil {
		t.Fatal("expected a playQueueByIndex element")
	}
	if pq.CurrentIndex != 1 {
		t.Fatalf("currentIndex = %d, want 1", pq.CurrentIndex)
	}
	if pq.Position != 77000 {
		t.Fatalf("position = %d, want 77000", pq.Position)
	}
	if len(pq.Entry) != 2 {
		t.Fatalf("len(entry) = %d, want 2", len(pq.Entry))
	}
	if pq.ChangedBy != "idx-client" {
		t.Fatalf("changedBy = %q, want idx-client", pq.ChangedBy)
	}
}

// "An out-of-range currentIndex obliges the server to return error code 10."
func TestSavePlayQueueByIndexRejectsOutOfRangeIndex(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	for _, idx := range []string{"2", "-1"} {
		q := url.Values{}
		q.Add("id", encodeTrackID(tracks[0].ID))
		q.Add("id", encodeTrackID(tracks[1].ID))
		q.Set("currentIndex", idx)
		body := getPlayQueueJSON(t, srv, "/rest/savePlayQueueByIndex.view?"+q.Encode())
		if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 10 {
			t.Fatalf("currentIndex=%s: expected error code 10, got %+v", idx, body.SubsonicResponse.Error)
		}
	}
}

func TestSavePlayQueueByIndexRequiresIndexWhenIdsPresent(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	body := getPlayQueueJSON(t, srv, "/rest/savePlayQueueByIndex.view?"+q.Encode())
	if body.SubsonicResponse.Error == nil || body.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("expected error code 10, got %+v", body.SubsonicResponse.Error)
	}
}

func TestSavePlayQueueByIndexWithoutParamsClears(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Set("currentIndex", "0")
	_ = getPlayQueueJSON(t, srv, "/rest/savePlayQueueByIndex.view?"+q.Encode())

	if body := getPlayQueueJSON(t, srv, "/rest/savePlayQueueByIndex.view"); body.SubsonicResponse.Status != "ok" {
		t.Fatalf("clear failed: %+v", body.SubsonicResponse.Error)
	}
	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueueByIndex.view")
	if got.SubsonicResponse.PlayQueueByIndex != nil {
		t.Fatal("expected no playQueueByIndex element after a clear")
	}
}

// Both variants read the same stored queue: a save through one is visible through
// the other. That is what makes the index-based pair an extension rather than a
// second queue.
func TestBothQueueVariantsShareOneStoredQueue(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Add("id", encodeTrackID(tracks[1].ID))
	q.Set("currentIndex", "1")
	q.Set("position", "3000")
	_ = getPlayQueueJSON(t, srv, "/rest/savePlayQueueByIndex.view?"+q.Encode())

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueue.view")
	pq := got.SubsonicResponse.PlayQueue
	if pq == nil {
		t.Fatal("expected the index-based save to be readable as an id-based queue")
	}
	if want := encodeTrackID(tracks[1].ID); pq.Current != want {
		t.Fatalf("current = %q, want %q", pq.Current, want)
	}
	if pq.Position != 3000 {
		t.Fatalf("position = %d, want 3000", pq.Position)
	}
}

// Ids that are not tracks (an album, a playlist) are not queue entries; accepting
// them would store rows that can never be rendered as songs.
func TestSavePlayQueueIgnoresNonTrackIds(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	db := s.DB()
	pl := model.Playlist{Name: "P"}
	db.Create(&pl)
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Add("id", encodePlaylistID(pl.ID))
	q.Set("current", encodeTrackID(tracks[0].ID))
	if body := getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode()); body.SubsonicResponse.Status != "ok" {
		t.Fatalf("save failed: %+v", body.SubsonicResponse.Error)
	}

	got := getPlayQueueJSON(t, srv, "/rest/getPlayQueue.view")
	if n := len(got.SubsonicResponse.PlayQueue.Entry); n != 1 {
		t.Fatalf("len(entry) = %d, want 1 — the playlist id must be dropped", n)
	}
}

// Queue entries are full Child objects, so a restoring client can rebuild its
// queue from one request instead of re-fetching every track.
func TestGetPlayQueueEntriesCarryStarredState(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	if err := s.Star("admin", "track", tracks[0].ID); err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, s)
	defer srv.Close()

	q := url.Values{}
	q.Add("id", encodeTrackID(tracks[0].ID))
	q.Add("id", encodeTrackID(tracks[1].ID))
	q.Set("current", encodeTrackID(tracks[0].ID))
	_ = getPlayQueueJSON(t, srv, "/rest/savePlayQueue.view?"+q.Encode())

	resp, err := http.Get(srv.URL + "/rest/getPlayQueue.view")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var raw struct {
		SubsonicResponse struct {
			PlayQueue struct {
				Entry []map[string]any `json:"entry"`
			} `json:"playQueue"`
		} `json:"subsonic-response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	entries := raw.SubsonicResponse.PlayQueue.Entry
	if len(entries) != 2 {
		t.Fatalf("len(entry) = %d, want 2", len(entries))
	}
	if _, ok := entries[0]["starred"]; !ok {
		t.Fatal("expected the starred track's entry to carry starred")
	}
	if _, ok := entries[1]["starred"]; ok {
		t.Fatal("expected the unstarred track's entry to omit starred")
	}
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

// queueRequest fires a /rest call as the given user against the
// identity-enabled test server.
func queueRequest(t *testing.T, srv *httptest.Server, user, path string) playQueueBody {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	req.Header.Set("X-Test-User", user)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body playQueueBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPlayQueueIsScopedToSessionUser(t *testing.T) {
	s := testStore(t)
	tracks := seedQueueTracks(t, s)
	srv := newTestServerWithIdentity(t, s)
	defer srv.Close()

	// demo saves a queue.
	save := "/rest/savePlayQueue?id=tr-" + itoa(tracks[0].ID) + "&current=tr-" + itoa(tracks[0].ID)
	if b := queueRequest(t, srv, "demo", save); b.SubsonicResponse.Status != "ok" {
		t.Fatalf("save as demo failed: %+v", b.SubsonicResponse)
	}

	// admin must NOT see demo's queue.
	b := queueRequest(t, srv, "admin", "/rest/getPlayQueue")
	if b.SubsonicResponse.PlayQueue != nil {
		t.Fatal("admin sees demo's queue: cross-user leak")
	}

	// demo still sees their own, with their username on it.
	b = queueRequest(t, srv, "demo", "/rest/getPlayQueue")
	if b.SubsonicResponse.PlayQueue == nil {
		t.Fatal("demo lost their own queue")
	}
	if got := b.SubsonicResponse.PlayQueue.Username; got != "demo" {
		t.Fatalf("expected username demo, got %q", got)
	}
}
