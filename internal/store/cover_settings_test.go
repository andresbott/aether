package store_test

import (
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestCoverSettingsSeedAndSet(t *testing.T) {
	s := testStore(t)

	got, err := s.GetCoverSettings([]string{"classic", "bauhaus"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DefaultStyles) != 2 || len(got.AvailableStyles) != 2 {
		t.Fatalf("first read should seed both sets, got %+v", got)
	}

	if err := s.SetCoverSettings(model.CoverSettings{DefaultStyles: []string{"rings"}, AvailableStyles: nil}); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetCoverSettings([]string{"classic", "bauhaus"}) // seed ignored now
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DefaultStyles) != 1 || got.DefaultStyles[0] != "rings" || len(got.AvailableStyles) != 0 {
		t.Fatalf("set/get mismatch: %+v", got)
	}
	if got.ID != 1 {
		t.Fatalf("singleton must stay id 1, got %d", got.ID)
	}
}
