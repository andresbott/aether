package libraries_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFilterOptionsListsConfiguredScanFolders proves scan_folders comes from
// h.Folders (the "Music"/"Books" set newTestHandler configures), in name
// order — the same proof shape as TestContractScanFoldersList: a schema-only
// check would pass just as happily with Folders never reaching the handler.
func TestFilterOptionsListsConfiguredScanFolders(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries/filter-options", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		ScanFolders []string `json:"scan_folders"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.ScanFolders) != 2 || got.ScanFolders[0] != "Books" || got.ScanFolders[1] != "Music" {
		t.Fatalf("expected scan_folders = [Books Music] (name order), got %v", got.ScanFolders)
	}
}

// TestFilterOptionsArraysAreNeverNull proves formats/genres/release_types
// are emitted as the JSON array "[]" when the catalog is empty, not null: a
// decoded Go slice cannot tell "[]" from "null" apart, so this checks the
// raw JSON text of each key instead.
func TestFilterOptionsArraysAreNeverNull(t *testing.T) {
	_, _, r := newTestHandler(t)
	req := httptest.NewRequest("GET", "/libraries/filter-options", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"formats", "genres", "release_types"} {
		v, ok := raw[key]
		if !ok {
			t.Fatalf("expected a %q key, body=%s", key, w.Body.String())
		}
		if string(v) != "[]" {
			t.Fatalf("expected %q to be the JSON array \"[]\" for an empty catalog, got %s", key, v)
		}
	}
}
