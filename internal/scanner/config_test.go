// internal/scanner/config_test.go
package scanner_test

import (
	"testing"

	"github.com/andresbott/aether/internal/scanner"
)

func TestParseMultiValueMode(t *testing.T) {
	tests := []struct {
		input string
		mode  scanner.MultiValueMode
		delim string
	}{
		{"none", scanner.MVNone, ""},
		{"multi", scanner.MVMulti, ""},
		{"delim ;", scanner.MVDelim, ";"},
		{"delim //", scanner.MVDelim, "//"},
		{"", scanner.MVNone, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, delim := scanner.ParseMultiValueMode(tt.input)
			if mode != tt.mode {
				t.Errorf("mode: got %v, want %v", mode, tt.mode)
			}
			if delim != tt.delim {
				t.Errorf("delim: got %q, want %q", delim, tt.delim)
			}
		})
	}
}

func TestApplyMultiValue(t *testing.T) {
	tests := []struct {
		name     string
		mode     scanner.MultiValueMode
		delim    string
		single   string
		multi    []string
		expected []string
	}{
		{"none", scanner.MVNone, "", "Rock; Pop", []string{"Rock", "Pop"}, []string{"Rock; Pop"}},
		{"multi", scanner.MVMulti, "", "Rock; Pop", []string{"Rock", "Pop"}, []string{"Rock", "Pop"}},
		{"delim", scanner.MVDelim, ";", "Rock;Pop", nil, []string{"Rock", "Pop"}},
		{"delim with spaces", scanner.MVDelim, ";", "Rock ; Pop", nil, []string{"Rock", "Pop"}},
		{"empty single none", scanner.MVNone, "", "", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.ApplyMultiValue(tt.mode, tt.delim, tt.single, tt.multi)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
