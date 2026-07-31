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
| Star/unstar, scrobble (play history) | Implemented | `subsonic/annotation.go`; single-user `StarredItem`. Only artists, albums, tracks (spec) and playlists (`playlistStar`) are starrable — `starrableTypes` drops genre/radio ids, which the spec has no way to surface. `starred` is emitted as an RFC3339 timestamp on every artist/album/song/playlist response via `subsonic/starred.go` (one batched `Store.StarredAt` per entity type), omitted when unstarred. Playlists also scrobble as a unit (`playlistScrobble` → `model.PlaylistPlay`); the web player scrobbles tracks at 50%/4min |
| setRating | Handler only — **not persisted** | `subsonic/annotation.go`; no rating column yet (TODO.md) |
| Internet radio CRUD | Implemented | `subsonic/radio.go`; writes currently open to all (admin-gating waits on user auth) |
| OpenSubsonic extensions | Implemented (12) | `subsonic/extensions.go` — musicFolderDefaultView/ShowArtists/Icon, albumList2Index, internetRadio/playlist/artist/genre CoverArt, playlistStar, playlistScrobble, playlistStats, discovery |
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
| Artist image from disk (`<collection>/<artist>/artist.jpg`) | Implemented | `scanner/artistimage.go` → `artist.ImagePath`, last fallback in `artistCoverMeta` ([scanning.md](scanning.md)) |
| Manual artist image search (same providers, user-picked MusicBrainz match) | Implemented, key-gated | `ArtistImageSearchDialog`, `/artists/image-preview` + `/artists/{id}/image-from-search` ([scanning.md](scanning.md)) |
| Metadata editor (tags, pictures, MusicBrainz identify) | Implemented | `handlers/metadata`, `internal/metadataedit`, `internal/identify` |
| Album identify (map a multi-file selection onto one release) | Implemented, key-gated | `POST /metadata/identify-album` → `internal/albumidentify`; `IdentifyAlbumDialog.vue` ([architecture.md](architecture.md)) |
| Shared per-file fingerprint cache (both identify flows) | Implemented | `identify.Cache` on the one `*identify.Identifier` both endpoints resolve through (`app/router/api_v1.go`); keyed path+size+mtime, LRU. Per-track and album identify reuse each other's fpcalc/AcoustID pass |
| MusicBrainz tracklist cache (album identify) | Implemented | `albumidentify.CachingReleaseLookup`, keyed by release MBID, wrapped **in front of** the 1 req/sec throttle — without it a repeat album identify still waited ~8s enriching options ([architecture.md](architecture.md)) |
| Genres from identify (both flows) | Implemented | Frontend-only: reuses `GET /musicbrainz/release-groups/{mbid}/genres`. Genre votes live on the release GROUP, not the release, so the identify response carries none; each dialog looks them up for the group the user settled on. No backend change — enriching server-side would cost a second throttled request per option (~8s per cold album identify, 7 of 8 wasted) and still leave the per-song flow uncovered |
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
| Identify result cache (reopening a dialog costs no lookup) | Implemented | `useIdentifyCache` (module-scoped LRU) + `useIdentifyRuns` (both flows, dialog state, aborts); Re-identify in both dialogs and the editor's Reload bypass it. Paired with the server-side fingerprint cache below |
| Release-group genre cache (both identify dialogs) | Implemented | `useReleaseGroupGenres` — module-scoped LRU keyed by release-group MBID, with in-flight dedupe so a folder of songs from one album costs ONE request against MusicBrainz's 1 req/sec throttle. Failures are not cached (a rate-limited lookup must stay retryable); an empty answer is |
| Favorites UI | Partial | album/artist/now-playing/song-detail toggles, playlist favorites (card/list/detail) and **grid-card hearts** (`AlbumCard`, `ArtistCard`, `PlaylistCard` — hover-revealed, pinned visible while favorited) all done on the `pi pi-heart(-fill)` icon with "Add to/Remove from favorites" wording; state survives a reload because `/rest` emits `starred`. Track-row toggles still missing (TODO.md) |
| Discovery ranked feed (albums + playlists in ONE ordering, grid+list) | Implemented | `DiscoveryView` + `DiscoveryFeedItem` + `useDiscoveryFeed`, served by the `discovery` extension (`getDiscovery`). Scored in the pure `internal/discovery` package: added-recency, favorite, log-saturating play count, play-recency, genre affinity from a recency-weighted taste profile, and seeded jitter — with a rediscovery quota at every 4th rank. Server owns the cross-type ranking and reports it as an absolute `rank` per entity, so the client merges the two per-type arrays with a sort. Infinite scroll (48/page), Refresh re-seeds. Grid renders through the shared `CardGrid`; list uses `AlbumRow`/`PlaylistRow` — never a card in a list, and never a second column mechanism. `reason` is still served (other clients may use it) but **deliberately not rendered**: on a lightly-played library nearly every item carries the same reason, so the badge was noise. Replaced the five themed shelves and `/discover/:section` |
| Search, genres, settings shell | Implemented | `SearchView`, `GenresView`/`GenreDetailView`, `SettingsLayout` |

## Not implemented (catalogued in TODO.md — read it first)

Last.fm scrobbling, DLNA/UPnP, jukebox/relay, transcoding, CUE sheets,
app icon/branding, `/api/v1` → `/admin` path reorg, per-scan cover-path
revalidation (known stale-cover bug with detailed root-cause notes),
`getPlaylists` N+1 fix, favorites schema rework.
