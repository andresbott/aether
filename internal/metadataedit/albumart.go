package metadataedit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

// errNoSelection is returned by ResolveAlbum when given no paths at all.
// Every caller either has an explicit, non-empty selection or has already
// fallen back to enumerating (and, failing that, seeding with) a browsed
// folder before calling in — so this is a caller bug, not a user-facing
// condition.
var errNoSelection = errors.New("metadataedit: at least one path is required")

// Album is a resolved, HTTP-agnostic view of one picture-editing selection:
// the absolute track files the caller selected (the embedded fan-out) and
// the distinct directories they live in (the folder fan-out). An album is
// not necessarily one folder — a multi-disc release is usually laid out as
// CD 1/, CD 2/ subfolders — so folder art is resolved across every directory
// the selection spans.
type Album struct {
	root   string
	tracks []string
	dirs   []string
}

// ResolveAlbum resolves relTrackPaths — library-relative, as returned by
// ListTracks or supplied by a client — against libRoot into an Album.
//
// Each entry becomes a track (contributing itself to Tracks() and its parent
// directory to Dirs()), unless it names an existing directory, in which case
// it contributes only that directory. That lets a caller seed a "browse this
// folder" album with no selected tracks — e.g. an empty or not-yet-populated
// album folder — by passing the folder's own path, so folder-art lookups
// still resolve against it.
//
// relTrackPaths must be non-empty: callers guarantee that (the picture
// endpoints fall back to enumerating the browsed folder, and ultimately to
// the folder itself, before calling in; a write endpoint validates a
// non-empty selection up front).
func ResolveAlbum(libRoot string, relTrackPaths []string) (Album, error) {
	if len(relTrackPaths) == 0 {
		return Album{}, errNoSelection
	}
	tracks := make([]string, 0, len(relTrackPaths))
	dirs := make([]string, 0, 2)
	seenDir := map[string]bool{}
	for _, rel := range relTrackPaths {
		abs, err := ResolveInLibrary(libRoot, rel)
		if err != nil {
			return Album{}, err
		}
		dir := abs
		if info, statErr := os.Stat(abs); statErr != nil || !info.IsDir() {
			// Not a directory (or nothing there — a stale/nonexistent
			// reference, tolerated exactly like today's lookups: it simply
			// never resolves to a picture downstream): treat it as a track.
			tracks = append(tracks, abs)
			dir = filepath.Dir(abs)
		}
		if !seenDir[dir] {
			seenDir[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return Album{root: filepath.Clean(libRoot), tracks: tracks, dirs: dirs}, nil
}

// Tracks returns the absolute paths of the selected track files — the
// embedded-picture fan-out — in selection order.
func (a Album) Tracks() []string { return a.tracks }

// Dirs returns the distinct directories the selection spans — the
// folder-picture fan-out — in first-seen order.
func (a Album) Dirs() []string { return a.dirs }

// relOf returns abs as a library-relative, forward-slash path.
func (a Album) relOf(abs string) string {
	return toForwardRel(a.root, abs)
}

// TypeSlots is one registry picture type's populated slots, as reported by
// Matrix. Types with no picture anywhere in the album are omitted.
type TypeSlots struct {
	Type  PictureType
	Slots []SlotState
}

// SlotState is one type+slot cell of the picture matrix.
type SlotState struct {
	Slot string // "embedded" | "folder"
	// Detail is the folder art's filename ("" for embedded).
	Detail string
	// Mixed marks a folder slot whose art is not the same in every directory
	// the album spans (a multi-disc release): one disc folder is missing it
	// or holds a different image.
	Mixed bool
	// PresentCount/TotalCount describe an embedded slot: how many of the
	// selected tracks carry this picture type, out of how many were
	// selected. Both are zero for a folder slot.
	PresentCount int
	TotalCount   int
	// Source locates this cell's representative image: the first embedded
	// track that carries it, or the first directory that holds the folder
	// art.
	Source Source
}

// Source is a bounded, library-relative locator for one resolved picture:
// the file, which slot it was found in, and which picture type it
// represents. It round-trips through Values/DecodeSource so the picture
// image endpoint can address a single resolved file without re-resolving an
// album selection.
type Source struct {
	RelPath string
	Slot    string
	TypeID  string
}

// sourceVersion tags the query encoding, so a later addressing scheme change
// (e.g. path to content-hash) is a detectable, additive version bump rather
// than a silent reinterpretation of old links.
const sourceVersion = "1"

// Values encodes s as the query parameters the picture image endpoint reads.
func (s Source) Values() url.Values {
	return url.Values{
		"file": {s.RelPath},
		"slot": {s.Slot},
		"type": {s.TypeID},
		"sv":   {sourceVersion},
	}
}

// DecodeSource decodes a Source from query parameters (the inverse of
// Values) and resolves its file against libRoot.
func DecodeSource(libRoot string, q url.Values) (absFile string, s Source, err error) {
	if v := q.Get("sv"); v != "" && v != sourceVersion {
		return "", Source{}, fmt.Errorf("metadataedit: unsupported source version %q", v)
	}
	s = Source{RelPath: q.Get("file"), Slot: q.Get("slot"), TypeID: q.Get("type")}
	abs, rerr := ResolveInLibrary(libRoot, s.RelPath)
	if rerr != nil {
		return "", Source{}, rerr
	}
	return abs, s, nil
}

// Matrix reports, for every registry picture type present somewhere in the
// album, which slots hold it: "embedded" (present_count of total_count
// selected tracks, one taglib properties read per track) and/or "folder"
// (the album's folder art, resolved across every directory the selection
// spans). ctx and r are accepted for symmetry with the package's other
// tag-reading operations; this resolution reads embedded pictures directly
// via taglib and stats folder files, neither of which needs a tags.Reader or
// cancellation.
func (a Album) Matrix(_ context.Context, _ tags.Reader) []TypeSlots {
	embeddedCount := map[string]int{}
	embeddedFirst := map[string]string{}
	for _, trackAbs := range a.tracks {
		images, err := ListEmbeddedPictures(trackAbs)
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, img := range images {
			if seen[img.Type] {
				continue
			}
			seen[img.Type] = true
			embeddedCount[img.Type]++
			if _, ok := embeddedFirst[img.Type]; !ok {
				embeddedFirst[img.Type] = trackAbs
			}
		}
	}

	out := make([]TypeSlots, 0, len(PictureTypes))
	for _, pt := range PictureTypes {
		var slots []SlotState
		if n := embeddedCount[pt.ID]; n > 0 {
			slots = append(slots, SlotState{
				Slot:         "embedded",
				PresentCount: n,
				TotalCount:   len(a.tracks),
				Source:       Source{RelPath: a.relOf(embeddedFirst[pt.ID]), Slot: "embedded", TypeID: pt.ID},
			})
		}
		if name, path, mixed, found := folderPictureAcross(a.dirs, pt); found {
			slots = append(slots, SlotState{
				Slot:   "folder",
				Detail: name,
				Mixed:  mixed,
				Source: Source{RelPath: a.relOf(path), Slot: "folder", TypeID: pt.ID},
			})
		}
		if len(slots) > 0 {
			out = append(out, TypeSlots{Type: pt, Slots: slots})
		}
	}
	return out
}

// Open resolves s to its bytes (embedded) or file path (folder), plus a
// fingerprint identifying the source content for cache invalidation. Exactly
// one of data/filePath is set on success.
func (a Album) Open(s Source) (data []byte, filePath, fingerprint string, err error) {
	return OpenSource(a.root, s)
}

// OpenSource is Open without an Album in hand: given a library root and a
// Source, it resolves and opens that one file directly. The picture image
// endpoint has exactly one resolved Source to serve and no reason to
// reconstruct a whole album selection just to open it.
func OpenSource(libRoot string, s Source) (data []byte, filePath, fingerprint string, err error) {
	abs, rerr := ResolveInLibrary(libRoot, s.RelPath)
	if rerr != nil {
		return nil, "", "", rerr
	}
	switch s.Slot {
	case "folder":
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return nil, "", "", statErr
		}
		fp := fmt.Sprintf("file|%s|%d|%d", abs, info.Size(), info.ModTime().UnixNano())
		return nil, abs, fp, nil
	case "embedded":
		imgData, ok, rerr2 := ReadEmbeddedPicture(abs, s.TypeID)
		if rerr2 != nil {
			return nil, "", "", rerr2
		}
		if !ok {
			return nil, "", "", fmt.Errorf("metadataedit: no %q picture embedded in %s", s.TypeID, filepath.Base(abs))
		}
		sum := sha256.Sum256(imgData)
		return imgData, "", "bytes|" + hex.EncodeToString(sum[:12]), nil
	default:
		return nil, "", "", fmt.Errorf("metadataedit: unknown slot %q", s.Slot)
	}
}

// DeleteFolderPicture removes the album's folder art of the given type from
// every directory the selection spans (mirroring the fan-out Matrix reports
// as one "folder" cell), independently per directory — a multi-disc album's
// discs may carry different filenames for "the same" loosely-matched front
// cover. Directories that do not carry the type are skipped.
func (a Album) DeleteFolderPicture(pt PictureType) error {
	for _, dir := range a.dirs {
		name := folderPictureName(dir, pt)
		if name == "" {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// folderPictureName returns the folder-art filename for a picture type in
// dir: front covers use the scanner's loose matching (folder.jpg, front.png,
// ... all count), every other type matches its exact <base>.jpg/png
// convention.
func folderPictureName(dir string, pt PictureType) string {
	if pt.ID == tags.FrontCoverType {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		return scanner.BestCover(names)
	}
	return FolderPictureName(dir, pt.FileBase)
}

// folderPictureAcross reports the album's folder art for a type across every
// directory the selection spans. detail/path come from the first directory
// that holds the picture (the album's representative image); mixed is true
// when the directories do not all hold the same bytes — i.e. one disc folder
// is missing the art or carries a different image — so the editor can flag
// it.
func folderPictureAcross(dirs []string, pt PictureType) (name, path string, mixed, ok bool) {
	var firstSum [sha256.Size]byte
	for _, dir := range dirs {
		n := folderPictureName(dir, pt)
		if n == "" {
			mixed = true // this folder lacks the album's art
			continue
		}
		p := filepath.Join(dir, n)
		sum, serr := fileSum(p)
		if !ok {
			name, path, firstSum, ok = n, p, sum, true
			continue
		}
		if serr != nil || sum != firstSum {
			mixed = true
		}
	}
	// A picture nowhere at all is simply absent, not mixed.
	if !ok {
		return "", "", false, false
	}
	return name, path, mixed, true
}

// fileSum returns the SHA-256 of a file's contents, used to tell whether the
// disc folders of one album hold the same image.
func fileSum(path string) ([sha256.Size]byte, error) {
	f, err := os.Open(path) //nolint:gosec // G304: path is built from a validated album directory plus a filename read from that directory
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer func() { _ = f.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}
