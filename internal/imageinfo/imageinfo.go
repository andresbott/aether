// Package imageinfo reports an image's storage-relevant metadata — pixel
// dimensions, encoded format and byte size — by decoding only the image header,
// never the full pixel data. It backs the metadata editor's "show image
// details" affordance for both on-disk images and images fetched from the
// online providers.
package imageinfo

import (
	"bytes"
	"image"
	"os"

	// Registered for image.DecodeConfig. These are the formats aether writes
	// (jpg/png) plus gif, which turns up as pre-existing folder art on disk.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Info is an image's displayable metadata. Format is the decoder name
// ("jpeg", "png", "gif"); it is "" — with zero dimensions — when the bytes are
// not a recognized image. Bytes is always the input length.
type Info struct {
	Width  int
	Height int
	Format string
	Bytes  int64
}

// Describe reports the metadata of an in-memory image. Undecodable bytes yield
// an Info carrying only Bytes, so a caller can still show the size of something
// that failed to decode.
func Describe(data []byte) Info {
	info := Info{Bytes: int64(len(data))}
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		info.Width, info.Height, info.Format = cfg.Width, cfg.Height, format
	}
	return info
}

// DescribeFile reports the metadata of an image file without reading it whole:
// it stats for the byte size and decodes only the header for dimensions and
// format. It errors only when the file cannot be opened; an openable but
// undecodable file yields an Info carrying just its size.
func DescribeFile(path string) (Info, error) {
	f, err := os.Open(path) //nolint:gosec // G304: callers pass a path already confined to the library root
	if err != nil {
		return Info{}, err
	}
	defer func() { _ = f.Close() }()

	info := Info{}
	if st, serr := f.Stat(); serr == nil {
		info.Bytes = st.Size()
	}
	if cfg, format, derr := image.DecodeConfig(f); derr == nil {
		info.Width, info.Height, info.Format = cfg.Width, cfg.Height, format
	}
	return info, nil
}
