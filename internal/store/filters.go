package store

// SearchFilter narrows search endpoints. All fields are optional.
type SearchFilter struct {
	// Scope, when non-zero, restricts results to entities with at least one
	// track the scope matches (see TrackScope).
	Scope TrackScope
}

// ArtistsFilter narrows artist-browsing endpoints.
type ArtistsFilter struct {
	Scope TrackScope
}

// StarredFilter narrows GetStarred.
type StarredFilter struct {
	Scope TrackScope
}
