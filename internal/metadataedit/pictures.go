package metadataedit

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/andresbott/aether/internal/fsx"
	"go.senan.xyz/taglib"
)

// ----- Embedded pictures -----
//
// Files can carry several attached pictures, each with a type ("Front Cover",
// "Back Cover", ...). The taglib API is index-based; these helpers translate
// between type IDs and indexes. When a file holds several pictures of the
// same type, writes replace the first one and deletes remove all of them.

// ListEmbeddedPictures returns the descriptors of every picture embedded in
// the audio file at path, in the file's own order.
func ListEmbeddedPictures(path string) ([]taglib.ImageDesc, error) {
	props, err := taglib.ReadProperties(path)
	if err != nil {
		return nil, fmt.Errorf("list embedded pictures: %w", err)
	}
	return props.Images, nil
}

// ReadEmbeddedPicture reads the first embedded picture of the given type.
// ok=false (with nil error) when the file has no picture of that type.
func ReadEmbeddedPicture(path, typeID string) ([]byte, bool, error) {
	images, err := ListEmbeddedPictures(path)
	if err != nil {
		return nil, false, err
	}
	for i, img := range images {
		if img.Type == typeID {
			data, rerr := taglib.ReadImageOptions(path, i)
			if rerr != nil {
				return nil, false, fmt.Errorf("read embedded picture: %w", rerr)
			}
			return data, true, nil
		}
	}
	return nil, false, nil
}

// WriteEmbeddedPicture writes data as the file's picture of the given type,
// replacing the first existing picture of that type (keeping its description)
// or appending a new one. An empty mimeType is sniffed from the data.
func WriteEmbeddedPicture(path, typeID string, data []byte, mimeType string) error {
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	images, err := ListEmbeddedPictures(path)
	if err != nil {
		return err
	}
	index := len(images) // out of range appends
	description := ""
	for i, img := range images {
		if img.Type == typeID {
			index = i
			description = img.Description
			break
		}
	}
	if err := taglib.WriteImageOptions(path, data, index, typeID, description, mimeType); err != nil {
		return fmt.Errorf("write embedded picture: %w", err)
	}
	return nil
}

// DeleteEmbeddedPicture removes every embedded picture of the given type.
// A file without that type is a no-op.
func DeleteEmbeddedPicture(path, typeID string) error {
	images, err := ListEmbeddedPictures(path)
	if err != nil {
		return err
	}
	// Deleting an index shifts the ones after it, so delete back to front.
	for i := len(images) - 1; i >= 0; i-- {
		if images[i].Type != typeID {
			continue
		}
		if err := taglib.WriteImageOptions(path, nil, i, "", "", ""); err != nil {
			return fmt.Errorf("delete embedded picture: %w", err)
		}
	}
	return nil
}

// ----- Folder pictures -----

// coverSiblingExts are every image extension a folder picture can take. A
// write or delete clears all of them for the base so a stale sibling in a
// different format never lingers. It matters most for artist portraits:
// internal/artistimage recognises all of these and ranks them equally, so a
// leftover artist.gif would keep being served in place of a new artist.jpg.
var coverSiblingExts = []string{"jpg", "jpeg", "png", "gif", "bmp", "webp"}

// WriteFolderPicture writes data to dir as <base>.<ext> (jpg or png),
// replacing any existing <base> image in any recognised format, and returns
// the written file's absolute path. The write is atomic (temp file + rename)
// so a served picture is never partial.
func WriteFolderPicture(dir, base, ext string, data []byte) (string, error) {
	ext = normCoverExt(ext)
	// Remove every other-format sibling so we never leave a stale <base>.<other>
	// that would shadow the file we are about to write.
	for _, e := range coverSiblingExts {
		if e != ext {
			_ = os.Remove(filepath.Join(dir, base+"."+e))
		}
	}
	final := filepath.Join(dir, base+"."+ext)

	tmp, err := fsx.CreateTemp(dir, base+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("write folder picture: temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write folder picture: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write folder picture: close: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write folder picture: rename: %w", err)
	}
	return final, nil
}

// FolderPictureName returns the filename of the folder picture with the given
// base (<base>.jpg or <base>.png, jpg preferred), or "" when absent. Front
// covers are looser than this — the handlers keep using scanner.BestCover for
// those so folder.jpg / front.png still count.
func FolderPictureName(dir, base string) string {
	for _, ext := range []string{"jpg", "png"} {
		name := base + "." + ext
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return name
		}
	}
	return ""
}

// DeleteFolderPicture removes the <base> folder picture in any recognised
// image format from dir. Missing files are a no-op.
func DeleteFolderPicture(dir, base string) error {
	for _, ext := range coverSiblingExts {
		p := filepath.Join(dir, base+"."+ext)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete folder picture: %w", err)
		}
	}
	return nil
}
