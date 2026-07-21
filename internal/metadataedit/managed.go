package metadataedit

import "strings"

// managedTagKeys are the raw tag keys (upper-cased) that the structured
// metadata editor owns: every alias the tag readers recognize for a field the
// edit form exposes. The raw editor must not write these — edits would fight
// with the form's own patch logic (positional MB-ID alignment, multi-value
// artist policies, year parsing).
var managedTagKeys = map[string]bool{
	"TITLE":                      true,
	"ALBUM":                      true,
	"ARTIST":                     true,
	"ALBUMARTIST":                true,
	"ALBUM_ARTIST":               true,
	"GENRE":                      true,
	"DATE":                       true,
	"YEAR":                       true,
	"ORIGINALDATE":               true,
	"TRACKNUMBER":                true,
	"TRACK":                      true,
	"DISCNUMBER":                 true,
	"DISC":                       true,
	"DISCSUBTITLE":               true,
	"TSST":                       true,
	"COMPILATION":                true,
	"TCMP":                       true,
	"MUSICBRAINZ_TRACKID":        true,
	"MUSICBRAINZ_ALBUMID":        true,
	"MUSICBRAINZ_RELEASEGROUPID": true,
	"MUSICBRAINZ_ARTISTID":       true,
	"MUSICBRAINZ_ALBUMARTISTID":  true,
}

// IsManagedTag reports whether a raw tag key belongs to the structured
// editor. Comparison is case-insensitive.
func IsManagedTag(key string) bool {
	return managedTagKeys[strings.ToUpper(strings.TrimSpace(key))]
}

// coverFrameIDs are the leading segments of unsupported-data descriptors that
// hold embedded cover art rather than junk: ID3v2 APIC, MP4 covr, ASF
// WM/Picture, APE Cover Art (Front)/(Back). Cover art has its own management
// UI; exposing these as deletable "hidden frames" would invite users to strip
// their covers by accident.
var coverFrameIDs = map[string]bool{
	"APIC":       true,
	"COVR":       true,
	"WM/PICTURE": true,
	"COVER ART":  true,
}

// IsCoverDescriptor reports whether an unsupported-data descriptor refers to
// embedded cover art. The descriptor's frame id is its segment before the
// first "/" (except WM/Picture, which contains one) — "APIC", "covr",
// "Cover Art (Front)".
func IsCoverDescriptor(descriptor string) bool {
	d := strings.ToUpper(strings.TrimSpace(descriptor))
	if d == "WM/PICTURE" {
		return true
	}
	id, _, _ := strings.Cut(d, "/")
	if coverFrameIDs[strings.TrimSpace(id)] {
		return true
	}
	// APE cover items are named "Cover Art (Front)", "Cover Art (Back)", ...
	return strings.HasPrefix(d, "COVER ART")
}
