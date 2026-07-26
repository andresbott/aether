// internal/tags/cover.go
package tags

import (
	"fmt"
	"strings"

	"go.senan.xyz/taglib"
)

// FrontCoverType is the canonical TagLib picture type of an album's front
// cover. A file can carry several attached pictures (back cover, disc, booklet,
// artist photo, ...) and only this one is the album's cover art — the rest must
// never be served as one.
const FrontCoverType = "Front Cover"

// ffprobeFrontCoverComment is how ffprobe labels a front cover: it exposes an
// attached picture's type in the video stream's "comment" tag, e.g.
// "Cover (front)", "Cover (back)", "Media (e.g. label side of CD)", "Other".
const ffprobeFrontCoverComment = "cover (front)"

// isFrontCoverComment reports whether an ffprobe attached-picture comment
// describes a front cover.
func isFrontCoverComment(comment string) bool {
	return strings.EqualFold(strings.TrimSpace(comment), ffprobeFrontCoverComment)
}

// hasFrontCover reports whether any of the file's attached pictures is a front
// cover. It scans every picture rather than just the first: taggers write them
// in arbitrary order, so a back cover can precede the front one.
func hasFrontCover(images []taglib.ImageDesc) bool {
	return frontCoverIndex(images) >= 0
}

// frontCoverIndex returns the index of the file's front-cover picture, or -1
// when it has none.
func frontCoverIndex(images []taglib.ImageDesc) int {
	for i, img := range images {
		if strings.EqualFold(strings.TrimSpace(img.Type), FrontCoverType) {
			return i
		}
	}
	return -1
}

// ReadFrontCover returns the bytes of the front-cover picture embedded in the
// audio file at path. ok=false (with a nil error) when the file carries no
// front cover — pictures of any other type (back cover, disc, booklet, artist
// photo) are never returned, since they are not the album's cover art.
func ReadFrontCover(path string) (data []byte, ok bool, err error) {
	props, err := taglib.ReadProperties(path)
	if err != nil {
		return nil, false, fmt.Errorf("read properties: %w", err)
	}
	idx := frontCoverIndex(props.Images)
	if idx < 0 {
		return nil, false, nil
	}
	data, err = taglib.ReadImageOptions(path, idx)
	if err != nil {
		return nil, false, fmt.Errorf("read front cover: %w", err)
	}
	if len(data) == 0 {
		return nil, false, nil
	}
	return data, true, nil
}
