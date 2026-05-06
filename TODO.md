# TODO

## Security

- [ ] Path traversal validation in `stream` and `getCoverArt` — validate that resolved file paths are within configured music directories before serving via `http.ServeFile`
- [ ] Authentication — implement real user auth with Subsonic token validation
- [ ] Gate radio station CRUD (create/update/delete) behind admin role once user management is implemented — reads stay open to all users, writes require admin

## API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly

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
- [ ] DLNA / UPnP endpoint — expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client

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
- [ ] `.nav-item.active` in `AppSidebar.vue` uses a hard-coded `#eef2ff` background instead of a `var(--app-*)` token — odd one out vs. the rest of the file
- [ ] `.view-placeholder` styles in `App.vue` look orphaned (the class isn't used in `App.vue`'s template) — verify whether any view actually consumes them, then either remove or move into the consuming view
- [ ] Clicking a row in the expanded `QueueSidebar` currently navigates to the song detail view — it should not switch views (keep the user on the current page); only the play/pause control should trigger playback
- [ ] Make songs in the `QueueSidebar` sortable via drag and drop — reorder tracks within the current queue by dragging
- [ ] Clicking a track row in the album view should not start playback — only the explicit play control on the row should play the song
- [ ] Album view should support multi-disc albums — group tracks by disc number (CD1, CD2, etc.) with per-disc headers
- [ ] Dark mode support — add a theme toggle and dark color tokens; audit hard-coded colors (e.g. `#eef2ff`, `#e0e7ff`) and migrate them to CSS variables
- [ ] Improve execution history and runtime task management — surface long-running jobs (scans, imports, transcodes) with progress, status, and cancel controls; persist recent run history for inspection
- [ ] Mute support in the player — clicking the volume icon toggles mute (preserving the previous volume level to restore on unmute)
- [ ] Keyboard shortcuts — play/pause (space), next/previous track, volume up/down, mute, seek, toggle queue sidebar; add a help overlay listing them
- [ ] "Now Playing" view should combine the current track detail (cover, metadata, lyrics, etc.) with a list view of the full queue below it — single landing page instead of separate song-detail + queue-sidebar split
- [ ] Create an app icon / logo — favicon, PWA icons (various sizes), and a wordmark for the topbar
