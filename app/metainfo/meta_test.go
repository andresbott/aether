package metainfo

import "testing"

func TestAcoustIDAppKey(t *testing.T) {
	tcs := []struct {
		version string
		want    string
	}{
		{"dev-build", "DslKKEx8DO"},
		{"0.1.0", "DslKKEx8DO"},
		{"v0.9.9", "DslKKEx8DO"},
		{"1.0.0", "40kOX9zexq"},
		{"v1.4.2", "40kOX9zexq"},
		{"garbage", "DslKKEx8DO"},
	}
	for _, tc := range tcs {
		if got := AcoustIDAppKey(tc.version); got != tc.want {
			t.Errorf("AcoustIDAppKey(%q) = %q, want %q", tc.version, got, tc.want)
		}
	}
}

func TestMajorVersion(t *testing.T) {
	tcs := []struct {
		version string
		want    int
	}{
		{"1.2.3", 1},
		{"v2.0.0", 2},
		{"0.4.0", 0},
		{"dev-build", 0},
		{"", 0},
	}
	for _, tc := range tcs {
		if got := majorVersion(tc.version); got != tc.want {
			t.Errorf("majorVersion(%q) = %d, want %d", tc.version, got, tc.want)
		}
	}
}
