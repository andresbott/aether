package coversettings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andresbott/aether/app/router/handlers/problems"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func newHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	return &Handler{Store: s, Gen: allstyles.New(), Problems: problems.New(false)}
}

func TestGetSeedsAllStyles(t *testing.T) {
	h := newHandler(t)
	r := mux.NewRouter()
	h.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/settings/generated-covers", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Styles    []struct{ Name, Label string } `json:"styles"`
		Default   []string                       `json:"default"`
		Available []string                       `json:"available"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	// Branch has 10 styles: classic/bauhaus/rings/waves/poster/remix/mosaic/halftone/liquid/lowpoly
	if len(body.Styles) != 10 || len(body.Default) != 10 || len(body.Available) != 10 {
		t.Fatalf("expected all ten styles seeded, got %+v", body)
	}
}

func TestPutRejectsUnknownAndEmptyDefault(t *testing.T) {
	h := newHandler(t)
	r := mux.NewRouter()
	h.Routes(r)

	do := func(payload string) int {
		req := httptest.NewRequest(http.MethodPut, "/settings/generated-covers", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	if do(`{"default":["nope"],"available":["classic"]}`) != http.StatusBadRequest {
		t.Fatal("unknown style should 400")
	}
	if do(`{"default":[],"available":["classic"]}`) != http.StatusBadRequest {
		t.Fatal("empty default should 400")
	}
	if do(`{"default":["bauhaus"],"available":[]}`) != http.StatusOK {
		t.Fatal("empty available is allowed")
	}
}
