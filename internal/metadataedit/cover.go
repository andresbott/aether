package metadataedit

import (
	"fmt"
	"os"
	"path/filepath"

	"go.senan.xyz/taglib"
)

// WriteEmbeddedCover writes data as the embedded front-cover image of the audio
// file at path, replacing any existing embedded picture. Passing empty data
// clears the embedded cover.
func WriteEmbeddedCover(path string, data []byte) error {
	if err := taglib.WriteImage(path, data); err != nil {
		return fmt.Errorf("write embedded cover: %w", err)
	}
	return nil
}

// WriteFolderCover writes data to dir as cover.<ext> (jpg or png), replacing any
// existing cover.jpg/cover.png, and returns the written file's absolute path.
// The write is atomic (temp file + rename) so a served cover is never partial.
func WriteFolderCover(dir, ext string, data []byte) (string, error) {
	ext = normCoverExt(ext)
	// Remove the sibling variant so we never leave both cover.jpg and cover.png.
	for _, e := range []string{"jpg", "png"} {
		if e != ext {
			_ = os.Remove(filepath.Join(dir, "cover."+e))
		}
	}
	final := filepath.Join(dir, "cover."+ext)

	tmp, err := os.CreateTemp(dir, "cover-*.tmp")
	if err != nil {
		return "", fmt.Errorf("write folder cover: temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write folder cover: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write folder cover: close: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write folder cover: rename: %w", err)
	}
	return final, nil
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
