package store

import (
	"time"

	"github.com/andresbott/aether/internal/discovery"
	"github.com/andresbott/aether/internal/model"
)

// DiscoveryFilter narrows the candidate pool. LibraryID nil means cross-library,
// matching every other list filter in this package.
type DiscoveryFilter struct {
	LibraryID *uint
}

// DiscoveryItem is one placed feed entry. It is deliberately flat and
// discovery-free so the handler layer can build its response without importing
// internal/discovery: exactly one of AlbumID / PlaylistID is set, per Kind.
type DiscoveryItem struct {
	Kind       string // "al" or "pl"
	AlbumID    uint
	PlaylistID uint
	Rank       int
	Reason     string
}

// discoveryPoolSize bounds the candidate pool. It is a CONSTANT, deliberately
// independent of size and offset: Rank sorts by score, so adding candidates moves
// the ones already there. A pool that grew with offset would let a high-scoring
// newcomer land inside ranks an earlier page already served, and the user would
// watch items repeat or vanish while scrolling. Fixed pool + fixed seed => every
// page of one feed scores an identical set, so absolute ranks are stable.
//
// The cost is a depth cap of ~41 pages of 48, far past where anyone scrolls a
// discovery surface. Past the end the feed returns nothing and the client's
// infinite scroll stops.
const discoveryPoolSize = 2000

// DiscoveryFeed gathers raw signals, scores them in internal/discovery, and
// returns the [offset, offset+size) window of the merged ranking.
//
// The pool is bounded rather than "every album": the union of three cheap
// orderings (newest, frequent, recent), every starred album, and a deterministic
// never-played sample for the rediscovery pool. Note there is no ORDER BY RANDOM()
// among them — it re-samples per call, which would hand consecutive pages partly
// disjoint sets and drift the ranks. Variety comes from the seeded jitter term
// instead: a new seed reorders the feed, and the same seed reproduces it exactly.
func (s *Store) DiscoveryFeed(owner string, size, offset int, seed int64, filter *DiscoveryFilter) ([]DiscoveryItem, error) {
	now := time.Now()

	// A failed taste query degrades to an empty profile rather than failing the
	// whole feed: the genre term contributes 0 and the other five still rank.
	// Same philosophy as the deliberately non-fatal star lookup in starred.go —
	// an enrichment signal must not take down the response it decorates.
	profile, err := s.tasteProfile(owner, now)
	if err != nil {
		profile = discovery.TasteProfile{}
	}
	albums, err := s.albumCandidates(owner, discoveryPoolSize, filter, now)
	if err != nil {
		return nil, err
	}
	playlists, err := s.playlistCandidates(owner)
	if err != nil {
		return nil, err
	}

	ranked := discovery.Rank(append(albums, playlists...), profile, seed, now, offset, size)
	out := make([]DiscoveryItem, 0, len(ranked))
	for _, r := range ranked {
		item := DiscoveryItem{
			Kind:   string(r.Kind),
			Rank:   r.Rank,
			Reason: string(r.Reason),
		}
		if r.Kind == discovery.KindAlbum {
			item.AlbumID = r.ID
		} else {
			item.PlaylistID = r.ID
		}
		out = append(out, item)
	}
	return out, nil
}

// tasteProfile builds the genre weight vector from play history inside the
// horizon plus every starred album's genres. The cutoff is a WHERE predicate so
// the horizon bounds how many play rows we scan and decode. Note play_histories
// indexes only track_id, so this is a bounded scan rather than an index seek —
// adding a played_at index would be a separate optimization.
func (s *Store) tasteProfile(owner string, now time.Time) (discovery.TasteProfile, error) {
	cutoff := now.Add(-discovery.TasteHorizon)

	type playRow struct {
		GenreID  uint      `gorm:"column:genre_id"`
		PlayedAt time.Time `gorm:"column:played_at"`
	}
	var playRows []playRow
	if err := s.db.Model(&model.PlayHistory{}).
		Select("track_genres.genre_id AS genre_id, play_histories.played_at AS played_at").
		Joins("JOIN track_genres ON track_genres.track_id = play_histories.track_id").
		Where("play_histories.played_at >= ?", cutoff).
		Scan(&playRows).Error; err != nil {
		return discovery.TasteProfile{}, err
	}
	plays := make([]discovery.GenrePlay, 0, len(playRows))
	for _, r := range playRows {
		plays = append(plays, discovery.GenrePlay{GenreID: r.GenreID, PlayedAt: r.PlayedAt})
	}

	type starRow struct {
		GenreID uint `gorm:"column:genre_id"`
	}
	var starRows []starRow
	if err := s.db.Model(&model.StarredItem{}).
		Select("album_genres.genre_id AS genre_id").
		Joins("JOIN album_genres ON album_genres.album_id = starred_items.item_id").
		Where("starred_items.owner = ? AND starred_items.item_type = ?", owner, "album").
		Scan(&starRows).Error; err != nil {
		return discovery.TasteProfile{}, err
	}
	stars := make([]discovery.GenreStar, 0, len(starRows))
	for _, r := range starRows {
		stars = append(stars, discovery.GenreStar{GenreID: r.GenreID})
	}

	return discovery.BuildTasteProfile(plays, stars, now), nil
}

// albumCandidates gathers the bounded album pool with its per-album signals in
// aggregate queries — never one query per album.
func (s *Store) albumCandidates(owner string, poolSize int, filter *DiscoveryFilter, now time.Time) ([]discovery.Candidate, error) {
	var libraryID *uint
	if filter != nil {
		libraryID = filter.LibraryID
	}

	ids, err := s.albumCandidateIDs(owner, poolSize, libraryID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	type albumRow struct {
		ID        uint      `gorm:"column:id"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	var rows []albumRow
	if err := s.db.Model(&model.Album{}).
		Select("id, created_at").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	starredAt, err := s.StarredAt(owner, "album", ids)
	if err != nil {
		return nil, err
	}
	plays, err := s.albumPlayStats(ids)
	if err != nil {
		return nil, err
	}
	genres, err := s.albumGenreIDs(ids)
	if err != nil {
		return nil, err
	}

	out := make([]discovery.Candidate, 0, len(rows))
	for _, r := range rows {
		c := discovery.Candidate{
			Kind:      discovery.KindAlbum,
			ID:        r.ID,
			CreatedAt: r.CreatedAt,
			GenreIDs:  genres[r.ID],
		}
		if ts, ok := starredAt[r.ID]; ok {
			c.StarredAt = &ts
		}
		if st, ok := plays[r.ID]; ok {
			c.PlayCount = st.PlayCount
			if !st.LastPlayed.IsZero() {
				last := st.LastPlayed
				c.LastPlayedAt = &last
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// albumCandidateIDs unions three cheap orderings, every starred album, and a
// deterministic never-played sample feeding the rediscovery pool.
//
// Every query here must be DETERMINISTIC for a given library state. In
// particular there is no "random" ordering: ORDER BY RANDOM() re-samples per
// call, so two pages of one feed would score partly disjoint sets and the ranks
// would drift under the user. Rediscovery variety comes from the seeded jitter
// term in scoring, not from the SQL.
func (s *Store) albumCandidateIDs(owner string, poolSize int, libraryID *uint) ([]uint, error) {
	seen := map[uint]bool{}
	var ids []uint

	add := func(batch []uint) {
		for _, id := range batch {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}

	// The three cheap orderings. GetAlbumList already implements each with the
	// same library filter, so reuse it rather than restating the SQL here.
	listFilter := &AlbumListFilter{LibraryID: libraryID, Owner: owner}
	for _, listType := range []string{"newest", "frequent", "recent"} {
		albums, err := s.GetAlbumList(listType, poolSize, 0, listFilter)
		if err != nil {
			return nil, err
		}
		batch := make([]uint, 0, len(albums))
		for i := range albums {
			batch = append(batch, albums[i].ID)
		}
		add(batch)
	}

	// Every starred album, regardless of pool size: a favorite must never be
	// crowded out of its own feed by an arbitrary cap.
	starredQ := s.db.Model(&model.StarredItem{}).
		Where("owner = ? AND item_type = ?", owner, "album")
	if libraryID != nil {
		starredQ = starredQ.Where(
			"EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = starred_items.item_id AND tracks.library_id = ?)",
			*libraryID,
		)
	}
	var starredIDs []uint
	if err := starredQ.Pluck("item_id", &starredIDs).Error; err != nil {
		return nil, err
	}
	add(starredIDs)

	// Never-played albums, which is what the rediscovery quota draws from. The
	// three orderings above are all play- or import-driven, so without this a
	// library whose newest imports are all played would surface no rediscovery
	// candidates at all. Ordered by id (stable, indexed) rather than randomly —
	// jitter does the shuffling at scoring time.
	unplayedQ := s.db.Model(&model.Album{}).
		Where("NOT EXISTS (SELECT 1 FROM play_histories JOIN tracks ON tracks.id = play_histories.track_id WHERE tracks.album_id = albums.id)")
	if libraryID != nil {
		unplayedQ = unplayedQ.Where(
			"EXISTS (SELECT 1 FROM tracks WHERE tracks.album_id = albums.id AND tracks.library_id = ?)",
			*libraryID,
		)
	}
	var unplayedIDs []uint
	if err := unplayedQ.Order("id ASC").Limit(poolSize).Pluck("id", &unplayedIDs).Error; err != nil {
		return nil, err
	}
	add(unplayedIDs)

	return ids, nil
}

// albumPlayStats returns play count and last-played per album in one grouped
// query, mirroring PlaylistStats' contract: never-played albums are absent.
func (s *Store) albumPlayStats(albumIDs []uint) (map[uint]PlaylistStat, error) {
	out := map[uint]PlaylistStat{}
	if len(albumIDs) == 0 {
		return out, nil
	}
	type row struct {
		AlbumID    uint   `gorm:"column:album_id"`
		PlayCount  int    `gorm:"column:play_count"`
		LastPlayed string `gorm:"column:last_played"`
	}
	var rows []row
	if err := s.db.Model(&model.PlayHistory{}).
		Select("tracks.album_id AS album_id, COUNT(*) AS play_count, datetime(MAX(play_histories.played_at)) AS last_played").
		Joins("JOIN tracks ON tracks.id = play_histories.track_id").
		Where("tracks.album_id IN ?", albumIDs).
		Group("tracks.album_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		stat := PlaylistStat{PlayCount: r.PlayCount}
		// Same degradation as PlaylistStats: an unparseable timestamp keeps the
		// count and leaves LastPlayed zero rather than failing the feed.
		if t, err := time.Parse("2006-01-02 15:04:05", r.LastPlayed); err == nil {
			stat.LastPlayed = t
		}
		out[r.AlbumID] = stat
	}
	return out, nil
}

// albumGenreIDs returns each album's genre ids in one query.
func (s *Store) albumGenreIDs(albumIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(albumIDs) == 0 {
		return out, nil
	}
	type row struct {
		AlbumID uint `gorm:"column:album_id"`
		GenreID uint `gorm:"column:genre_id"`
	}
	var rows []row
	if err := s.db.Table("album_genres").
		Select("album_id, genre_id").
		Where("album_id IN ?", albumIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.AlbumID] = append(out[r.AlbumID], r.GenreID)
	}
	return out, nil
}

// playlistCandidates takes every playlist: a library holds few enough of them
// that sampling would cost more clarity than it saves query time.
func (s *Store) playlistCandidates(owner string) ([]discovery.Candidate, error) {
	playlists, err := s.GetPlaylists(owner)
	if err != nil {
		return nil, err
	}
	if len(playlists) == 0 {
		return nil, nil
	}
	ids := make([]uint, 0, len(playlists))
	for i := range playlists {
		ids = append(ids, playlists[i].ID)
	}
	starredAt, err := s.StarredAt(owner, "playlist", ids)
	if err != nil {
		return nil, err
	}
	stats, err := s.PlaylistStats(ids)
	if err != nil {
		return nil, err
	}
	genres, err := s.playlistGenreIDs(ids)
	if err != nil {
		return nil, err
	}

	out := make([]discovery.Candidate, 0, len(playlists))
	for i := range playlists {
		pl := playlists[i]
		c := discovery.Candidate{
			Kind:      discovery.KindPlaylist,
			ID:        pl.ID,
			CreatedAt: pl.CreatedAt,
			GenreIDs:  genres[pl.ID],
		}
		if ts, ok := starredAt[pl.ID]; ok {
			c.StarredAt = &ts
		}
		if st, ok := stats[pl.ID]; ok {
			c.PlayCount = st.PlayCount
			if !st.LastPlayed.IsZero() {
				last := st.LastPlayed
				c.LastPlayedAt = &last
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// GetAlbumsByIDs loads albums by primary key with the associations albumToMap
// needs, in one query. It exists so the discovery handler can render a page of
// ranked albums without issuing one GetAlbum per row. Tracks are deliberately
// not preloaded: callers pair this with AlbumTrackStats for songCount/duration.
// The returned order is unspecified — callers hold the ranking and reorder.
func (s *Store) GetAlbumsByIDs(ids []uint) ([]model.Album, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var albums []model.Album
	err := s.db.
		Preload("Artists").
		Preload("Genres").
		Where("id IN ?", ids).
		Find(&albums).Error
	return albums, err
}

// playlistGenreIDs reaches a playlist's genres through its tracks. Duplicates
// are kept: a playlist that is three-quarters one genre should read as mostly
// that genre, which is exactly what averaging over the repeated ids gives.
func (s *Store) playlistGenreIDs(playlistIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	if len(playlistIDs) == 0 {
		return out, nil
	}
	type row struct {
		PlaylistID uint `gorm:"column:playlist_id"`
		GenreID    uint `gorm:"column:genre_id"`
	}
	var rows []row
	if err := s.db.Table("playlist_tracks").
		Select("playlist_tracks.playlist_id AS playlist_id, track_genres.genre_id AS genre_id").
		Joins("JOIN track_genres ON track_genres.track_id = playlist_tracks.track_id").
		Where("playlist_tracks.playlist_id IN ?", playlistIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.PlaylistID] = append(out[r.PlaylistID], r.GenreID)
	}
	return out, nil
}
