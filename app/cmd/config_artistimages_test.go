package cmd

import "testing"

func TestArtistImagesDefaults(t *testing.T) {
	// By default no provider API keys are configured, so the fetch-artist-images
	// task reports a "not configured" message when run (it is always registered).
	if defaultCfg.ArtistImages.FanartApiKey != "" {
		t.Fatalf("FanartApiKey should default to empty, got %q", defaultCfg.ArtistImages.FanartApiKey)
	}
	if defaultCfg.ArtistImages.TheAudioDBApiKey != "" {
		t.Fatalf("TheAudioDBApiKey should default to empty, got %q", defaultCfg.ArtistImages.TheAudioDBApiKey)
	}
}
