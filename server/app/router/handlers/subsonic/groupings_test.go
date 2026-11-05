package subsonic

import (
	"net/http"
	"testing"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
)

type groupingsEnvelope struct {
	SubsonicResponse struct {
		Status    string `json:"status"`
		Groupings struct {
			Index []struct {
				Name     string `json:"name"`
				Grouping []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					CoverArt   string `json:"coverArt"`
					AlbumCount int    `json:"albumCount"`
					SongCount  int    `json:"songCount"`
					Year       int    `json:"year"`
				} `json:"grouping"`
			} `json:"index"`
		} `json:"groupings"`
		Grouping struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			AlbumCount int    `json:"albumCount"`
			SongCount  int    `json:"songCount"`
			Year       int    `json:"year"`
			Album      []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"album"`
		} `json:"grouping"`
		Album struct {
			Grouping struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"grouping"`
		} `json:"album"`
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	} `json:"subsonic-response"`
}

func seedGroupings(t *testing.T, s *store.Store) (saga model.Album) {
	t.Helper()
	lib := model.Library{Name: "L1", Filters: scanFolderFilter("L1")}
	if err := s.DB().Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	add := func(name string, year int, folder, grouping string) model.Album {
		al := model.Album{Name: name, NameNorm: name, AlbumArtistNorm: "x", Year: year}
		if err := s.DB().Create(&al).Error; err != nil {
			t.Fatal(err)
		}
		tr := model.Track{AlbumID: al.ID, ScanFolder: folder, Grouping: grouping, Duration: 60, Filename: name, FilePath: "/" + folder + "/" + name}
		if err := s.DB().Create(&tr).Error; err != nil {
			t.Fatal(err)
		}
		return al
	}
	add("Part 2", 2005, "L1", "Saga")
	saga = add("Part 1", 2001, "L1", "Saga")
	add("Elsewhere", 2003, "L2", "Saga")
	add("Solo", 1999, "L2", "Anthology")
	add("Plain", 2000, "L1", "")
	if err := s.ReconcileGroupings(); err != nil {
		t.Fatal(err)
	}
	return saga
}

func TestGetGroupingsIndexAndScope(t *testing.T) {
	s := testStore(t)
	saga := seedGroupings(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	var all groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGroupings.view", &all)
	idx := all.SubsonicResponse.Groupings.Index
	if len(idx) != 2 || idx[0].Name != "A" || idx[1].Name != "S" {
		t.Fatalf("index letters = %+v", idx)
	}
	g := idx[1].Grouping[0]
	if g.Name != "Saga" || g.AlbumCount != 3 || g.SongCount != 3 || g.Year != 2001 || g.CoverArt != g.ID {
		t.Fatalf("Saga entry = %+v", g)
	}

	var scoped groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGroupings.view?musicFolderId=1", &scoped)
	idx = scoped.SubsonicResponse.Groupings.Index
	if len(idx) != 1 || idx[0].Grouping[0].AlbumCount != 2 {
		t.Fatalf("scoped index = %+v", idx)
	}

	// getAlbum names the album's grouping.
	var album groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getAlbum.view?id="+encodeAlbumID(saga.ID), &album)
	if got := album.SubsonicResponse.Album.Grouping; got.Name != "Saga" || got.ID != g.ID {
		t.Fatalf("album grouping = %+v", got)
	}
}

func TestGetGroupingAlbumsInOrder(t *testing.T) {
	s := testStore(t)
	seedGroupings(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	var all groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGroupings.view", &all)
	id := all.SubsonicResponse.Groupings.Index[1].Grouping[0].ID

	var one groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGrouping.view?id="+id, &one)
	g := one.SubsonicResponse.Grouping
	if g.Name != "Saga" || len(g.Album) != 3 || g.Album[0].Name != "Part 1" || g.Album[1].Name != "Elsewhere" || g.Album[2].Name != "Part 2" {
		t.Fatalf("grouping = %+v", g)
	}
	if g.AlbumCount != 3 || g.SongCount != 3 || g.Year != 2001 {
		t.Fatalf("grouping counts = %+v", g)
	}

	var scoped groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGrouping.view?id="+id+"&musicFolderId=1", &scoped)
	if len(scoped.SubsonicResponse.Grouping.Album) != 2 {
		t.Fatalf("scoped albums = %+v", scoped.SubsonicResponse.Grouping.Album)
	}

	for _, bad := range []string{"id=al-1", "id=gr-999"} {
		var e groupingsEnvelope
		decodeJSON(t, srv.URL+"/rest/getGrouping.view?"+bad, &e)
		if e.SubsonicResponse.Status != "failed" {
			t.Fatalf("%s: expected an error, got %q", bad, e.SubsonicResponse.Status)
		}
	}
	var missing groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGrouping.view", &missing)
	if missing.SubsonicResponse.Error.Code != 10 {
		t.Fatalf("missing id error = %+v", missing.SubsonicResponse.Error)
	}
}

func TestGroupingCoverArtResolves(t *testing.T) {
	s := testStore(t)
	seedGroupings(t, s)
	srv := newTestServer(t, s)
	defer srv.Close()

	var all groupingsEnvelope
	decodeJSON(t, srv.URL+"/rest/getGroupings.view", &all)
	id := all.SubsonicResponse.Groupings.Index[1].Grouping[0].ID

	resp := getCover(t, srv.URL+"/rest/getCoverArt.view?id="+id, "image/jpeg")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cover status = %d", resp.StatusCode)
	}
}

func TestRootViewsOfferGroupingsOnlyWhenEnabled(t *testing.T) {
	s := testStore(t)
	srv := newTestServer(t, s)
	defer srv.Close()

	if err := s.SaveCatalogSettings(model.CatalogSettings{SplitViews: true, ShowGroupings: true}); err != nil {
		t.Fatal(err)
	}
	var body musicFoldersBody
	decodeJSON(t, srv.URL+"/rest/getMusicFolders.view", &body)
	got := body.SubsonicResponse.MusicFolders.Catalog.Views
	if len(got) != 4 || got[2] != "groupings" || got[3] != "albums" {
		t.Fatalf("root views = %v", got)
	}
}
