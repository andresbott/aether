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
| Play queue (cross-device session resume) | Implemented | `subsonic/playqueue.go` — `savePlayQueue`/`getPlayQueue` (spec) plus `savePlayQueueByIndex`/`getPlayQueueByIndex` (`indexBasedQueue` extension). **One stored queue per owner, backed by an INDEX not a track id** (`model.PlayQueue` + `PlayQueueEntry`): a queue may hold the same track twice, so `current` as an id cannot say which copy is playing — the id-based save resolves to the first matching slot. Both variants read/write the same row. A save with no `id` clears the queue (spec's clear call). `GetPlayQueue` drops tracks deleted since the save, shifts `CurrentIndex` onto the survivors, and **zeroes `PositionMs` when the current track itself is gone** — an offset measured in one track is meaningless in another |
| OpenSubsonic extensions | Implemented (13) | `subsonic/extensions.go` — musicFolderDefaultView/ShowArtists/Icon, albumList2Index, internetRadio/playlist/artist/genre CoverArt, playlistStar, playlistScrobble, playlistStats, discovery, indexBasedQueue |
| XML responses (`f=xml`) | Rejected by design (for now) | middleware in `subsonic/subsonic.go` returns an error; TODO.md tracks third-party client compat |
| Transcoding | Not implemented | TODO.md (FFmpeg planned) |
| getArtistInfo/getAlbumInfo, getTopSongs/getSimilarSongs | Not implemented | TODO.md — needs external metadata / play-history analysis |
| Podcasts, jukebox | Not implemented, out of scope this pass | TODO.md. Podcasts needs a feed poller + episode download state; jukebox needs local audio output in-process |
| Bookmarks (`createBookmark`/`getBookmarks`/`deleteBookmark`) | Not implemented — **deliberately superseded for resume** | `savePlayQueue` already carries a `position` for the current track, so cross-device resume-within-song needs no bookmark; adding one would have meant two writes per tick for the same fact. Bookmarks remain a real gap only for **per-track offsets that outlive a queue replacement** (audiobooks, long DJ sets). If added later they must not become a second source of truth for the current track's position — the play queue owns that |
| Sharing, chat | **Not planned — rejected by design** | Do not implement `createShare`/`updateShare`/`deleteShare`/`getShares` or `addChatMessage`/`getChatMessages`. Sharing means public unauthenticated links (a `/share.php`-style HTML landing page) that deliberately bypass auth — not something this server should expose. Chat is a server-wide message wall with no rooms or delivery, vestigial in the ecosystem and meaningless on a single-user server. Clients discover both are absent the normal way (no extension advertised, `getShares`/`getChatMessages` simply not routed) |
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
| Cross-device queue restore (SPA) | Implemented | `useQueueSync` + `usePlayer.restoreSession`, armed in `PlayerLayout` (`restore()` then `start()`, in that order — starting first lets a debounced local save race the incoming server state). **Saves on edits, not continuously**: queue/currentIndex watchers, debounced 500ms so a drag or multi-select removal is one request. Four triggers in all, and only these — `currentTime` is deliberately never watched (it changes ~4×/sec): (1) a queue/slot **edit**, debounced; (2) a **30s ticker** while playing (paused ⇒ no tick, since the pause already saved); (3) **pause**, which bypasses the debounced path on purpose — that path skips saves whose queue *shape* is unchanged, and on a pause only the position moved — and which is likewise gated on `playedHere`, because a restore lands paused and must not be echoed back; (4) **`pagehide`**, via `subsonicClient.savePlayQueueBeacon` → `navigator.sendBeacon`, because a normal `fetch` is cancelled during unload — but **only if this tab actually played** (`playedHere`). That guard is load-bearing: `pagehide` also fires on *reload*, so an unconditional beacon let a tab that had merely restored someone else's queue write that restored offset straight back, landing ~10ms after and overwriting a newer save from another browser. Symptom was: pause in Firefox, open Chrome, and Firefox's pause point is gone. The handler reads `player.isPlaying` directly as well as the flag, since the flag's watcher is async and a tab closed moments after playback began would otherwise lose the write. Without (3) and (4) a pause-then-close lost everything back to the last tick — up to 30s. `pagehide` rather than `beforeunload`: it also fires on mobile and back-forward-cache navigations. Seeking still does not save on its own; the next trigger carries it. Uses the **index-based** endpoints for the duplicate-track reason above. `syncedSignature` (ids + slot) is what stops a restore echoing back as a save; a plain "restoring" flag does not work, because the watchers a restore trips fire on the *next tick*, after the flag clears. Restore is deliberately **paused** (browsers block autoplay) and the offset applies to the restored current track only — stepping to any other slot goes through `loadTrack` and starts at 0. `start()`/`stop()` are symmetric: `stop()` calls every `watch` stop handle and unbinds the `pagehide` listener, because `PlayerLayout` unmounts and remounts on each trip through `/settings` and a one-way "bound" flag left the sync permanently dead after the first unmount |
| Player (queue, shuffle/repeat, gapless-ish preload) | Implemented | `composables/usePlayer.ts` — dual `<audio>` preload swap; true Web-Audio gapless deferred (`docs/gapless-playback-web-audio.md`); mute + keyboard shortcuts TODO |
| Now Playing / queue edit / drag-to-queue | Implemented | `QueueView`, `useQueueEdit`, drag composables |
| Playlists UI (inline rename, batched track edit) | Implemented | `PlaylistsView`, `PlaylistDetailView` |
| Radio UI | Implemented | `RadioView`, `RadioStationDetailView` (`/radio/new` create mode) |
| Metadata editor UI | Implemented | `/settings/metadata`, `useEditSession` staged overlays |
| Identify result cache (reopening a dialog costs no lookup) | Implemented | `useIdentifyCache` (module-scoped LRU) + `useIdentifyRuns` (both flows, dialog state, aborts); Re-identify in both dialogs and the editor's Reload bypass it. Paired with the server-side fingerprint cache below |
| Release-group genre cache (both identify dialogs) | Implemented | `useReleaseGroupGenres` — module-scoped LRU keyed by release-group MBID, with in-flight dedupe so a folder of songs from one album costs ONE request against MusicBrainz's 1 req/sec throttle. Failures are not cached (a rate-limited lookup must stay retryable); an empty answer is |
| Favorites UI | Partial | album/artist/now-playing/song-detail toggles, playlist favorites (card/list/detail) and **grid-card hearts** (`AlbumCard`, `ArtistCard`, `PlaylistCard` — hover-revealed, pinned visible while favorited) all done on the `pi pi-heart(-fill)` icon with "Add to/Remove from favorites" wording; state survives a reload because `/rest` emits `starred`. Track-row toggles still missing (TODO.md) |
| Discovery ranked feed (albums + playlists in ONE ordering, grid+list) | Implemented | `DiscoveryFeed` (the body: states, grid/list, sentinel) + `DiscoveryFeedItem` + `useDiscoveryFeed`, framed by the **Discover tab in `LibraryView`** — the default tab on the root `/library` only, since the ranking is cross-collection and a per-library feed would be meaningless (no `musicFolderId` is ever passed, and a folder deep-linked to `#discover` falls back to its albums). There is **no standalone `/discover` route or `DiscoveryView`**: they were folded into the tab so the nav has one door to the feed, and the sidebar's `Library` entry (compass icon, flat with the per-folder entries) is that door. `LibraryView` and `DiscoveryFeed` both call `useDiscoveryFeed` and hit the same query cache entry, so the tab's count summary costs no extra request. Served by the `discovery` extension (`getDiscovery`). Scored in the pure `internal/discovery` package: added-recency, favorite, log-saturating play count, play-recency, genre affinity from a recency-weighted taste profile, and seeded jitter — with a rediscovery quota at every 4th rank. Server owns the cross-type ranking and reports it as an absolute `rank` per entity, so the client merges the two per-type arrays with a sort. Infinite scroll (48/page). The client seeds from the current **12-hour window** (`discoverySeedForTime`), captured once per mount, so the feed holds still for twelve hours and then rolls on its own — there is deliberately no Refresh action and no way to reshuffle by hand. Grid renders through the shared `CardGrid`; list uses `AlbumRow`/`PlaylistRow` — never a card in a list, and never a second column mechanism. `reason` is still served (other clients may use it) but **deliberately not rendered**: on a lightly-played library nearly every item carries the same reason, so the badge was noise. Replaced the five themed shelves and `/discover/:section` |
| Search, genres, settings shell | Implemented | `SearchView`, `GenresView`/`GenreDetailView`, `SettingsLayout` |

## Not implemented (catalogued in TODO.md — read it first)

Last.fm scrobbling, DLNA/UPnP, jukebox/relay, transcoding, CUE sheets,
app icon/branding, `/api/v1` → `/admin` path reorg, per-scan cover-path
revalidation (known stale-cover bug with detailed root-cause notes),
`getPlaylists` N+1 fix, favorites schema rework.
