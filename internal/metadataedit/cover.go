package metadataedit

import "github.com/andresbott/aether/internal/tags"

// Front-cover conveniences over the typed picture helpers in pictures.go.

// WriteEmbeddedCover writes data as the embedded front-cover image of the
// audio file at path, replacing any existing front cover. Passing empty data
// removes the front cover(s).
func WriteEmbeddedCover(path string, data []byte) error {
	if len(data) == 0 {
		return DeleteEmbeddedPicture(path, tags.FrontCoverType)
	}
	return WriteEmbeddedPicture(path, tags.FrontCoverType, data, "")
}

// WriteFolderCover writes data to dir as cover.<ext> (jpg or png), replacing
// any existing cover.jpg/cover.png, and returns the written file's absolute
// path.
func WriteFolderCover(dir, ext string, data []byte) (string, error) {
	return WriteFolderPicture(dir, "cover", ext, data)
}

func normCoverExt(ext string) string {
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext != "jpg" && ext != "png" {
		ext = "jpg"
	}
	return ext
}
