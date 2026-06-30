package cmd

import "testing"

func TestArtistImagesDefaults(t *testing.T) {
	if defaultCfg.ArtistImages.Enabled {
		t.Fatal("ArtistImages should default to disabled")
	}
}
