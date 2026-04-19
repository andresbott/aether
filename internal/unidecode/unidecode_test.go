// internal/unidecode/unidecode_test.go
package unidecode_test

import (
	"testing"

	"github.com/andresbott/aether/internal/unidecode"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Björk", "bjork"},
		{"Sigur Rós", "sigur ros"},
		{"Radiohead", "radiohead"},
		{"METALLICA", "metallica"},
		{"café", "cafe"},
		{"日本語", "ri ben yu"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := unidecode.Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
