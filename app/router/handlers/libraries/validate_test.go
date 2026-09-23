package libraries

import (
	"slices"
	"strings"
	"testing"

	"github.com/andresbott/aether/internal/model"
)

func TestValidateName(t *testing.T) {
	if err := ValidateName(""); err == nil {
		t.Fatal("empty name should fail")
	}
	if err := ValidateName("   "); err == nil {
		t.Fatal("whitespace-only name should fail")
	}
	if err := ValidateName(strings.Repeat("a", 201)); err == nil {
		t.Fatal("201-char name should fail (max 200)")
	}
	if err := ValidateName("Main"); err != nil {
		t.Fatalf("simple name should pass, got %v", err)
	}
}

func TestValidateViews(t *testing.T) {
	type v = model.LibraryView
	cases := []struct {
		name         string
		in           []v
		want         []v
		wantPointers []string
	}{
		{"one view", []v{"releases"}, []v{"releases"}, nil},
		{"put in display order", []v{"releases", "discover", "artists"}, []v{"discover", "artists", "releases"}, nil},
		{"each view once", []v{"artists", "artists"}, []v{"artists"}, nil},
		{"none", []v{}, nil, []string{"/views"}},
		{"every unknown value, by index", []v{"albums", "artists", "Songs"}, nil, []string{"/views/0", "/views/2"}},
		{"case-sensitive", []v{"Discover"}, nil, []string{"/views/0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, problems := ValidateViews(c.in)
			pointers := make([]string, 0, len(problems))
			for _, p := range problems {
				pointers = append(pointers, p.Pointer)
			}
			if !slices.Equal(pointers, c.wantPointers) {
				t.Fatalf("problem pointers = %v, want %v", pointers, c.wantPointers)
			}
			if !slices.Equal(got, c.want) {
				t.Fatalf("views = %v, want %v", got, c.want)
			}
		})
	}
}

func TestValidateDefaultView(t *testing.T) {
	views := []model.LibraryView{model.ViewArtists, model.ViewReleases}
	cases := []struct {
		in model.LibraryView
		ok bool
	}{
		{"", true}, // the first view, picked by the caller
		{"artists", true},
		{"releases", true},
		{"discover", false}, // a real view, but not one of this library's
		{"albums", false},
		{"Artists", false}, // case-sensitive
	}
	for _, c := range cases {
		err := ValidateDefaultView(c.in, views)
		gotOK := err == nil
		if gotOK != c.ok {
			t.Errorf("%q: expected ok=%v, got err=%v", c.in, c.ok, err)
		}
	}
}

func TestValidateIcon(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", true}, // empty coerced to default by caller
		{"folder", true},
		{"queue_music", true},
		{"10k", true},
		{"folder-open", false}, // kebab-case is the old PrimeIcons shape
		{"Folder", false},      // case-sensitive
		{"folder!", false},
		{"folder open", false},
		{"_folder", false},
		{"folder_", false},
		{"folder__open", false},
		{strings.Repeat("a", 101), false},
	}
	for _, c := range cases {
		err := ValidateIcon(c.in)
		gotOK := err == nil
		if gotOK != c.ok {
			t.Errorf("%q: expected ok=%v, got err=%v", c.in, c.ok, err)
		}
	}
}
