# TODO

## Security

- [ ] Path traversal validation in `stream` and `getCoverArt` — validate that resolved file paths are within configured music directories before serving via `http.ServeFile`
- [ ] Authentication — implement real user auth with Subsonic token validation

## API Completeness

- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.)
- [ ] Transcoding — identify formats browsers can't play natively and add FFmpeg transcoding
- [ ] setRating persistence — add rating column to tracks/albums when needed
- [ ] getArtistInfo / getAlbumInfo — external metadata (MusicBrainz bios, similar artists)
- [ ] getTopSongs / getSimilarSongs — requires external data or play history analysis
- [ ] Podcasts, Radio, Bookmarks, Sharing, Chat, Jukebox — not in scope for this pass

## Frontend — Wire SPA to Subsonic API

### Phase 1: Make existing views work
- [ ] Auto-connect without credentials (backend has no auth)
- [ ] Library view — albums and artists fetch and display correctly
- [ ] Album view — shows cover, metadata, track list; play/queue works
- [ ] Artist view — shows artist info and discography
- [ ] Playlists view — lists playlists from API
- [ ] Player — streaming playback works end-to-end
- [ ] Home view — shows now playing or sensible empty state

### Phase 2: Fill in missing features
- [ ] Search UI — add search bar that uses the existing search composable
- [ ] Playlist create/edit — create new playlists, add/remove songs
- [ ] Song view — display track detail from queue
- [ ] Favorites/starring — star/unstar from album/artist/track views
- [ ] Songs tab in Library — fetch and display all songs

## Integration

- [ ] Last.fm scrobbling — explore Last.fm API integration for external scrobbling
- [ ] Multi-user — single-user for now; playlist owner column is pre-wired

## Performance

- [ ] `getPlaylists` N+1 queries — each playlist triggers separate count and duration queries; consider a single annotated query
- [ ] `albumToMap` missing `songCount`/`duration` when tracks are not preloaded — album list endpoints don't preload tracks, so these fields are absent in list responses
- [ ] cover art is extacted from the file on the fly, it might perform better if we extract at scannnig 

## Data Integrity

- [ ] Cleanup orphaned `playlist_tracks`, `starred_items`, and `play_histories` when tracks/albums/artists are deleted during scan cleanup
- [ ] Use `errors.Is(err, gorm.ErrRecordNotFound)` in `FindOrCreateArtists` and `FindOrCreateAlbum` to distinguish not-found from real DB errors

## Frontend Layout — Pre-existing items surfaced during topbar refactor

Surfaced by reviews of the topbar refactor (2026-04-26) but pre-existing — not introduced by that work. To review:

- [ ] `PlayerControls` is `position: fixed` (not a flex row inside `App.vue`'s shell), so the bottom of `.main-content` sits *under* the player. Any content scrolled to the bottom is partially obscured. Fix candidates: give `.app-container` a `padding-bottom: var(--app-player-height)`, or restructure `PlayerControls` into the column flex
- [ ] `QueueSidebar` height uses `calc(100vh - var(--app-player-height))` — works but is now slightly inconsistent with the new body-row layout. Could be simplified to `height: 100%` to match `AppSidebar` and let flex govern
- [ ] `.nav-item.active` in `AppSidebar.vue` uses a hard-coded `#eef2ff` background instead of a `var(--app-*)` token — odd one out vs. the rest of the file
- [ ] `.view-placeholder` styles in `App.vue` look orphaned (the class isn't used in `App.vue`'s template) — verify whether any view actually consumes them, then either remove or move into the consuming view
