package metadata

import "github.com/andresbott/aether/internal/imageinfo"

// imageMetaDTO is an image's displayable metadata — pixel dimensions, encoded
// format and byte size — served alongside both stored images (the artist image,
// the album's picture cells) and downloaded candidates, so the editor can show
// something like "1000 × 1000 · JPEG · 245 KB".
type imageMetaDTO struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Bytes  int64  `json:"bytes"`
}

// toImageMeta maps the imageinfo domain value onto its transport DTO.
func toImageMeta(info imageinfo.Info) imageMetaDTO {
	return imageMetaDTO{
		Width:  info.Width,
		Height: info.Height,
		Format: info.Format,
		Bytes:  info.Bytes,
	}
}
