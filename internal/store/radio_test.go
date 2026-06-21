package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestCreateInternetRadioStation(t *testing.T) {
	s := testStore(t)
	st, err := s.CreateInternetRadioStation("BBC R1", "http://example.com/r1", "http://bbc.co.uk")
	if err != nil {
		t.Fatal(err)
	}
	if st.ID == 0 || st.Name != "BBC R1" || st.StreamURL != "http://example.com/r1" || st.HomepageURL != "http://bbc.co.uk" {
		t.Fatalf("unexpected: %+v", st)
	}
}

func TestCreateInternetRadioStationNoHomepage(t *testing.T) {
	s := testStore(t)
	st, err := s.CreateInternetRadioStation("Nova", "http://example.com/nova", "")
	if err != nil {
		t.Fatal(err)
	}
	if st.HomepageURL != "" {
		t.Fatalf("expected empty homepage, got %q", st.HomepageURL)
	}
}

func TestGetInternetRadioStations(t *testing.T) {
	s := testStore(t)
	_, _ = s.CreateInternetRadioStation("Zulu", "http://z", "")
	_, _ = s.CreateInternetRadioStation("Alpha", "http://a", "")
	stations, err := s.GetInternetRadioStations()
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(stations))
	}
	if stations[0].Name != "Alpha" || stations[1].Name != "Zulu" {
		t.Fatalf("expected name-ASC order, got %s, %s", stations[0].Name, stations[1].Name)
	}
}

func TestGetInternetRadioStation(t *testing.T) {
	s := testStore(t)
	created, _ := s.CreateInternetRadioStation("R1", "http://r1", "")
	got, err := s.GetInternetRadioStation(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Name != "R1" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestGetInternetRadioStationNotFound(t *testing.T) {
	s := testStore(t)
	if _, err := s.GetInternetRadioStation(9999); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateInternetRadioStation(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("Old", "http://old", "")
	if err := s.UpdateInternetRadioStation(st.ID, "New", "http://new", "http://home"); err != nil {
		t.Fatal(err)
	}
	var loaded model.InternetRadioStation
	s.DB().First(&loaded, st.ID)
	if loaded.Name != "New" || loaded.StreamURL != "http://new" || loaded.HomepageURL != "http://home" {
		t.Fatalf("unexpected: %+v", loaded)
	}
}

func TestUpdateInternetRadioStationNotFound(t *testing.T) {
	s := testStore(t)
	err := s.UpdateInternetRadioStation(9999, "X", "http://x", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteInternetRadioStation(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("Temp", "http://t", "")
	if err := s.DeleteInternetRadioStation(st.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	s.DB().Model(&model.InternetRadioStation{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestDeleteInternetRadioStationNotFound(t *testing.T) {
	s := testStore(t)
	err := s.DeleteInternetRadioStation(9999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateInternetRadioStationCoverPath(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("R1", "http://r1", "")
	if err := s.UpdateInternetRadioStationCoverPath(st.ID, "/tmp/covers/1.png"); err != nil {
		t.Fatal(err)
	}
	var loaded model.InternetRadioStation
	s.DB().First(&loaded, st.ID)
	if loaded.CoverPath != "/tmp/covers/1.png" {
		t.Fatalf("expected cover path to be set, got %q", loaded.CoverPath)
	}
}

func TestUpdateInternetRadioStationCoverPathClear(t *testing.T) {
	s := testStore(t)
	st, _ := s.CreateInternetRadioStation("R1", "http://r1", "")
	_ = s.UpdateInternetRadioStationCoverPath(st.ID, "/tmp/covers/1.png")
	if err := s.UpdateInternetRadioStationCoverPath(st.ID, ""); err != nil {
		t.Fatal(err)
	}
	var loaded model.InternetRadioStation
	s.DB().First(&loaded, st.ID)
	if loaded.CoverPath != "" {
		t.Fatalf("expected cover path cleared, got %q", loaded.CoverPath)
	}
}

func TestUpdateInternetRadioStationCoverPathNotFound(t *testing.T) {
	s := testStore(t)
	err := s.UpdateInternetRadioStationCoverPath(9999, "/x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
