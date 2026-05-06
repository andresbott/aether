package store

// SearchFilter narrows search endpoints. All fields are optional.
type SearchFilter struct {
	// LibraryID, when non-nil, restricts results to entities with at least
	// one track in that library.
	LibraryID *uint
}

// ArtistsFilter narrows artist-browsing endpoints.
type ArtistsFilter struct {
	LibraryID *uint
}

// StarredFilter narrows GetStarred.
type StarredFilter struct {
	LibraryID *uint
}
