# Features — status and where each lives

Check here before adding a capability — the gap may already be catalogued with
a chosen direction (usually in `TODO.md`). Statuses verified against the code
2026-07-22. See [architecture.md](architecture.md) for the package map.

## Music serving (`/rest`, see [subsonic-api.md](subsonic-api.md))

| Feature | Status | Where |
|---|---|---|
| Browsing (folders/indexes/artists/albums/songs/genres) | Implemented | `app/router/handlers/subsonic/browsing.go`, `genres.go` |
| Album lists, random songs, songs by genre, starred | Implemented | `subsonic/lists.go` (`getStarred2` only; no `getStarred`) |
| Search | Implemented | `subsonic/search.go` (`search3`) |
| Streaming | Implemented, original file only | `subsonic/media.go` — `http.ServeFile` with range support; no transcoding |
| Cover art (albums/artists/playlists/radio/genres) | Implemented | `subsonic/media.go` + `assetstore` + `covergen` fallback (deterministic generated covers) |
| Playlists CRUD + cover upload | Implemented | `subsonic/playlists.go` |
| Star/unstar, scrobble (play history) | Implemented | `subsonic/annotation.go`; single-user `StarredItem` |
| setRating | Handler only — **not persisted** | `subsonic/annotation.go`; no rating column yet (TODO.md) |
| Internet radio CRUD | Implemented | `subsonic/radio.go`; writes currently open to all (admin-gating waits on user auth) |
| OpenSubsonic extensions | Implemented (8) | `subsonic/extensions.go` — musicFolderDefaultView/ShowArtists/Icon, albumList2Index, internetRadio/playlist/artist/genre CoverArt |
| XML responses (`f=xml`) | Rejected by design (for now) | middleware in `subsonic/subsonic.go` returns an error; TODO.md tracks third-party client compat |
| Transcoding | Not implemented | TODO.md (FFmpeg planned) |
| getArtistInfo/getAlbumInfo, getTopSongs/getSimilarSongs | Not implemented | TODO.md — needs external metadata / play-history analysis |
| Podcasts, bookmarks, sharing, chat, jukebox | Not implemented, out of scope this pass | TODO.md |
| CUE sheet albums | Not implemented — design recorded | `docs/cue-playing.md` (2026-07-22) |

## Security & users

| Feature | Status | Where |
|---|---|---|
| Authentication | **Not implemented — everything is open** | Web SPA calls `/rest` with no credentials (`subsonicClient.initWithDefaults()` skips auth). Direction is decided: session auth for the SPA, PATs for third-party Subsonic clients via the `userauth` library — full model incl. the Authelia trusted-header deployment in [authentication.md](authentication.md). Do not invent a different scheme. |
| Multi-user | Not implemented | Single-user; `Playlist.Owner` pre-wired; per-user star schema chosen in TODO.md |
| Path traversal validation on stream/getCoverArt | Open TODO | Serving paths come from the trusted DB; the metadata editor validates via `metadataedit.ResolveInLibrary` — the `/rest` media handlers do not yet |

## Server administration (`/api/v1`)

| Feature | Status | Where |
|---|---|---|
| Libraries CRUD + folder browse | Implemented | `handlers/libraries` (path change wipes the library's tracks) |
| Scanning (incremental + full) | Implemented | tasks `scan` / `scan-full` → `internal/scanner`; see [scanning.md](scanning.md) |
| Task runner, schedules, execution history + logs + cancel | Implemented | `handlers/tasks`, `internal/taskrunner` |
| Artist image fetching | Implemented, key-gated | task `fetch-artist-images`, `internal/artistimage` (fanart.tv → TheAudioDB chain) |
| Metadata editor (tags, pictures, MusicBrainz identify) | Implemented | `handlers/metadata`, `internal/metadataedit`, `internal/identify` |
| Radio-browser station import | Implemented | `handlers/radiobrowser` (server-side proxy) |
| Prometheus metrics | Implemented, opt-in | separate observability server, off unless `Observability.Enabled` (`:9009` default), `handlers/admin.go` |

## Web UI (see [frontend.md](frontend.md))

| Feature | Status | Where |
|---|---|---|
| Library browse (albums/artists, grid+list, alphabet rail) | Implemented | `LibraryView` + `components/library/*`; grouped artist-header layout still wanted (TODO.md) |
| Player (queue, shuffle/repeat, gapless-ish preload) | Implemented | `composables/usePlayer.ts` — dual `<audio>` preload swap; true Web-Audio gapless deferred (`docs/gapless-playback-web-audio.md`); mute + keyboard shortcuts TODO |
| Now Playing / queue edit / drag-to-queue | Implemented | `QueueView`, `useQueueEdit`, drag composables |
| Playlists UI (inline rename, batched track edit) | Implemented | `PlaylistsView`, `PlaylistDetailView` |
| Radio UI | Implemented | `RadioView`, `RadioStationDetailView` (`/radio/new` create mode) |
| Metadata editor UI | Implemented | `/settings/metadata`, `useEditSession` staged overlays |
| Favorites UI | Partial | album/artist/now-playing toggles done; track-row stars, grid badges, starred browse section missing (TODO.md) |
| Search, genres, settings shell | Implemented | `SearchView`, `GenresView`/`GenreDetailView`, `SettingsLayout` |

## Not implemented (catalogued in TODO.md — read it first)

Last.fm scrobbling, DLNA/UPnP, jukebox/relay, transcoding, CUE sheets,
app icon/branding, `/api/v1` → `/admin` path reorg, per-scan cover-path
revalidation (known stale-cover bug with detailed root-cause notes),
`getPlaylists` N+1 fix, favorites schema rework.
