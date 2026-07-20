package scanner

import "testing"

func TestDecodeExcludePatterns(t *testing.T) {
	tcs := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{name: "empty string", in: "", want: 0},
		{name: "valid json list", in: `["^\\..", "Thumbs\\.db"]`, want: 2},
		{name: "invalid json", in: `not json`, wantErr: true},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeExcludePatterns(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeExcludePatterns: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("expected %d patterns, got %d", tc.want, len(got))
			}
		})
	}
}

func TestCompileExcludes(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		got, err := compileExcludes("")
		if err != nil || got != nil {
			t.Fatalf("expected nil, nil; got %v, %v", got, err)
		}
	})
	t.Run("valid patterns", func(t *testing.T) {
		got, err := compileExcludes(`["^\\..", "Thumbs\\.db"]`)
		if err != nil {
			t.Fatalf("compileExcludes: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 regexps, got %d", len(got))
		}
		if !got[0].MatchString(".hidden") {
			t.Fatal("expected first pattern to match .hidden")
		}
	})
	t.Run("invalid regexp is skipped", func(t *testing.T) {
		got, err := compileExcludes(`["[invalid", "ok"]`)
		if err != nil {
			t.Fatalf("compileExcludes: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 regexp after skipping invalid, got %d", len(got))
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		if _, err := compileExcludes(`not json`); err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}
