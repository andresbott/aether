package metadataedit

// PictureType is one kind of attached picture the editor manages. ID is the
// canonical TagLib pictureType string as stored in the files; FileBase is the
// filename base used both for folder art (<base>.jpg/png) and for the named
// assetstore entry.
type PictureType struct {
	ID       string
	Label    string
	FileBase string
}

// PictureTypes is the curated, ordered registry of picture types the editor
// offers. Mirrored in webui/src/lib/pictureTypes.ts — keep the two in sync.
var PictureTypes = []PictureType{
	{ID: "Front Cover", Label: "Front cover", FileBase: "cover"},
	{ID: "Back Cover", Label: "Back cover", FileBase: "back"},
	{ID: "Media", Label: "Media (disc)", FileBase: "disc"},
	{ID: "Leaflet Page", Label: "Booklet", FileBase: "booklet"},
	{ID: "Artist", Label: "Artist", FileBase: "artist"},
	{ID: "Band", Label: "Band", FileBase: "band"},
	{ID: "Illustration", Label: "Illustration", FileBase: "illustration"},
	{ID: "Other", Label: "Other", FileBase: "other"},
}

// PictureTypeByID looks a registry entry up by its canonical ID.
func PictureTypeByID(id string) (PictureType, bool) {
	for _, pt := range PictureTypes {
		if pt.ID == id {
			return pt, true
		}
	}
	return PictureType{}, false
}
