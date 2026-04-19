# Gonic - Architecture & Features Analysis

## Overview

Gonic is a self-hosted music streaming server written in Go that implements the Subsonic API. It provides a lightweight, FLOSS alternative to Subsonic/Airsonic for streaming a personal music library to 
compatible clients (DSub, Ultrasonic, Submariner, etc.). It also supports podcasts, internet radio, and jukebox playback.

---

## Architecture

### Technology Stack

| Component        | Technology                                                       |
|------------------|------------------------------------------------------------------|
| Language         | Go 1.25                                                          |
| Database         | SQLite 3 (via GORM v1 ORM + `mattn/go-sqlite3` or WASM SQLite)  |
| HTTP Framework   | Go stdlib `net/http` + `http.ServeMux`                           |
| Session Store    | `gormstore` (session storage in the database)                    |
| Tag Reading      | `go.senan.xyz/taglib` (native taglib bindings) or `ffprobe`      |
| Transcoding      | FFmpeg (invoked as subprocess)                                   |
| Jukebox          | MPV (controlled via IPC socket with `mpvipc`)                    |
| Podcast Parsing  | `gofeed` RSS parser                                              |
| Templates        | Go `html/template` with Sprig functions                          |
| File Watching    | `fsnotify`                                                       |
| Concurrency      | `errgroup` for lifecycle management of background goroutines     |
| Config           | CLI flags + env vars + config file (`flagconf`)                  |

### Package Structure

```
gonic/
  cmd/gonic/         Main entrypoint, CLI flag parsing, service wiring
  db/                Data models (GORM), migrations, DB operations
  scanner/           Music library scanner (walk, tag read, DB populate, cleanup)
  transcode/         Transcoding profiles (FFmpeg), caching transcoder
  jukebox/           MPV-based server-side playback via IPC
  podcast/           Podcast RSS feed management, episode download/purge
  playlist/          M3U playlist file store (filesystem-backed)
  scrobble/          Scrobbler interface (Last.fm + ListenBrainz)
  lastfm/            Last.fm API client (scrobbling, artist/album info, similar)
  listenbrainz/      ListenBrainz API client (scrobbling)
  tags/              Tag reading abstraction (taglib, ffprobe backends)
  infocache/         Cached artist/album info from Last.fm
    artistinfocache/ Background refresh of artist biographies/images/similar
    albuminfocache/  Background refresh of album notes
  server/
    ctrladmin/       Admin web UI controller (HTML templates, session auth)
    ctrlsubsonic/    Subsonic API controller (XML/JSON/JSONP responses)
      spec/          Subsonic response type definitions and constructors
      specid/        Typed IDs (track, album, artist, podcast, etc.)
      specidpaths/   Resolve spec IDs to filesystem paths
      params/        Subsonic request parameter parsing
  handlerutil/       HTTP middleware (logging, CORS, path trimming)
  fileutil/          Filesystem helpers (safe filenames, path prefix checks)
  mockfs/            Test filesystem mocking
  deps/              Build-tag-switched dependencies (native vs WASM SQLite/taglib)
```

### Request Flow

1. HTTP request arrives at `net/http.ServeMux`
2. Routes split into `/admin/` (web UI) and `/rest/` (Subsonic API)
3. Middleware chain: HTTP logging -> CORS -> `.view` suffix trimming
4. **Subsonic path**: parameter parsing -> required param check -> user auth (token or password) -> handler -> XML/JSON/JSONP response
5. **Admin path**: session middleware -> user session check -> admin role check (for admin routes) -> handler -> HTML template render

### Authentication

- **Subsonic API**: Two modes per the Subsonic spec:
  - Token auth: MD5(password + salt) sent as `t` and `s` params
  - Password auth: plaintext or hex-encoded (`enc:`) password as `p` param
  - Username always required as `u` param
- **Admin UI**: Cookie-based sessions stored in SQLite via `gormstore`
  - Separate user/admin role middleware chains

### Database Design

SQLite with GORM v1. Key entities and relationships:

- **Artist** - name, unicode-decoded name, star/rating per user, cached info (biography, image, similar artists, top tracks)
- **Album** - path-based (root_dir + left_path + right_path), parent album hierarchy, cover art, tag metadata, compilation flag, release type, disc titles, star/rating per user
- **Track** - filename, tag metadata (title, artist, number, disc, year, lyrics, brainz ID), audio properties (length, bitrate, size), ReplayGain values (track/album gain+peak), embedded cover flag, star/rating per user
- **Genre** - many-to-many with both tracks and albums
- **Artist relationships** - album_artists, track_artists, artist_appearances (many-to-many)
- **User** - name, password, admin flag, avatar, Last.fm session, ListenBrainz URL+token
- **Play** - per-user per-album play count and last play time
- **PlayQueue** - per-user play queue state (current track, position, items)
- **Bookmark** - per-user position bookmarks for tracks or podcast episodes
- **TranscodePreference** - per-user per-client transcode profile selection
- **Podcast** - RSS URL, title, description, image, auto-download setting, episodes
- **PodcastEpisode** - title, audio URL, status (downloading/skipped/deleted/completed/error), file metadata
- **InternetRadioStation** - stream URL, name, homepage
- **Setting** - key-value store (Last.fm API key/secret, last scan time, session key)
- **AlbumInfo** - cached album notes/MusicBrainz ID/Last.fm URL
- **ArtistInfo** - cached biography/images/similar artists/top tracks

---

## Features

### Music Library Management

- **Multiple music directories**: Support for multiple `-music-path` arguments, each with optional aliases (`alias -> /path`)
- **Automatic scanning**: Configurable interval scan (`-scan-interval` in minutes)
- **Scan at startup**: Optional initial scan on server boot (`-scan-at-start-enabled`)
- **Filesystem watcher**: Real-time detection of new/changed/deleted files using `fsnotify` (`-scan-watcher-enabled`), with batched re-scans (10s debounce)
- **Full and incremental scans**: Incremental by default (only re-reads tracks modified since last scan); full scan available via admin UI or API
- **Exclude patterns**: Regex-based file/folder exclusion (`-exclude-pattern`)
- **Symlink support**: Follows symbolic links during scanning
- **Tag reading**: Reads metadata via taglib (native) or ffprobe (fallback), including:
  - Title, artist, album artist, album, genre, year, track number, disc number, disc subtitle
  - MusicBrainz recording ID, release ID
  - Lyrics (embedded, including LRC synced format)
  - ReplayGain values (track gain/peak, album gain/peak)
  - Compilation flag, release type 
  - Original date / date for year extraction
- **Multi-value tag support**: Configurable handling for genre, artist, and album artist:
  - `none`: single value (default)
  - `multi`: native multi-value tags (e.g., multiple ARTIST frames)
  - `delim <sep>`: split single value by delimiter
- **Unicode normalization**: Latin-equivalent (`unidecode`) fields for search support
- **Cover art detection**: External cover files (heuristic-based via `coverparse`) and embedded covers in audio files (configurable)
- **Cleanup**: Automatic removal of orphaned tracks, albums, artists, genres, and bookmarks after scan

### Subsonic API Compatibility

Implements the Subsonic/OpenSubsonic API at `/rest/`. Supports XML, JSON, and JSONP response formats.

**OpenSubsonic Extensions**:
- `transcodeOffset` v1
- `formPost` v1
- `songLyrics` v1

**System**:
- `ping` - server alive check
- `getLicense` - always returns valid
- `getOpenSubsonicExtensions` - lists supported extensions
- `getUser` - current user info with roles
- `getMusicFolders` - list configured music directories
- `getScanStatus` - scan progress/state
- `startScan` - trigger library scan

**Browsing by Tags** (ID3):
- `getArtists` - alphabetical artist index
- `getArtist` - artist details with albums
- `getAlbum` - album details with tracks
- `getAlbumList2` - album lists (random, newest, frequent, recent, starred, by year, by genre, alphabetical, highest rated)
- `search3` - search artists, albums, tracks
- `getStarred2` - starred items
- `getArtistInfo2` - artist biography, image, similar artists (from Last.fm cache)
- `getAlbumInfo2` - album notes (from Last.fm cache)
- `getTopSongs` - artist top tracks (from Last.fm)
- `getSimilarSongs` / `getSimilarSongs2` - similar tracks (from Last.fm)
- `getGenres` - genre list with counts

**Browsing by Folder**:
- `getIndexes` - folder-based artist index
- `getMusicDirectory` - folder contents
- `getAlbumList` - same list types as tag-based
- `search2` - folder-based search
- `getStarred` - folder-based starred items
- `getArtistInfo` - folder-based artist info

**Media Retrieval**:
- `stream` / `download` - audio streaming with transcoding support and seek offset
- `getCoverArt` - cover art retrieval (file-based or embedded, with resizing/caching)
- `getAvatar` - user avatar images
- `getLyrics` - lyrics by artist+title (prefers unsynced)
- `getLyricsBySongId` - structured lyrics by track ID (synced .lrc, unsynced .txt, embedded tag lyrics)
- `getSong` - single track details
- `getRandomSongs` - random tracks with optional genre/year/folder filters
- `getSongsByGenre` - tracks by genre

**User Interaction**:
- `star` / `unstar` - star/unstar artists, albums, tracks
- `setRating` - rate items (1-5 scale)
- `scrobble` - scrobble plays to Last.fm and ListenBrainz (with now-playing support)

**Playlists**:
- `getPlaylists` - list playlists
- `getPlaylist` - playlist details with tracks
- `createPlaylist` / `updatePlaylist` - create or modify playlists
- `deletePlaylist` - remove playlist
- Backed by M3U files on disk with custom `#GONIC-` attributes (name, comment, public flag)
- Per-user playlist directories (`<playlists-path>/<user-id>/`)

**Play Queue**:
- `savePlayQueue` / `getPlayQueue` - persist/restore play queue state across clients

**Bookmarks**:
- `getBookmarks` / `createBookmark` / `deleteBookmark` - position bookmarks for tracks and podcast episodes

**Podcasts**:
- `getPodcasts` - list podcast channels with episodes
- `getNewestPodcasts` - newest episodes across all podcasts
- `createPodcastChannel` - add podcast by RSS URL
- `refreshPodcasts` - refresh all feeds
- `deletePodcastChannel` - remove podcast and files
- `deletePodcastEpisode` - remove single episode
- `downloadPodcastEpisode` - queue episode for download

**Internet Radio**:
- `getInternetRadioStations` - list stations
- `createInternetRadioStation` - add station
- `updateInternetRadioStation` - modify station
- `deleteInternetRadioStation` - remove station

**Jukebox** (server-side playback):
- `jukeboxControl` with actions: `get`, `set`, `add`, `clear`, `remove`, `start`, `stop`, `skip`, `setGain`

### Transcoding

- **FFmpeg-based**: Shells out to FFmpeg for on-the-fly transcoding
- **Built-in profiles**:
  - `mp3` (128kbps), `mp3_320` (320kbps), `mp3_rg` (128kbps with ReplayGain)
  - `opus` (96kbps), `opus_rg`, `opus_car` (96kbps with aggressive ReplayGain preamp)
  - `opus_128`, `opus_128_rg`, `opus_128_car` (128kbps variants)
  - `opus_192` (192kbps)
  - `PCM16le` (raw 16-bit PCM for jukebox)
- **ReplayGain application**: Profiles with `_rg` suffix apply ReplayGain normalization during transcoding; `_car` profiles use louder preamp (+15dB) for noisy environments
- **Transcode caching**: Disk-based cache with configurable size limit (`-transcode-cache-size` MB) and LRU eviction (`-transcode-eject-interval` minutes)
- **Per-user per-client preferences**: Users can set different transcode profiles for different client apps
- **Seek support**: `transcodeOffset` OpenSubsonic extension for seeking within transcoded streams

### Scrobbling

- **Last.fm integration**:
  - Scrobble plays and now-playing updates
  - Artist info (biography, images, similar artists, top tracks)
  - Album info (notes, MusicBrainz ID)
  - Track similarity
  - Loved tracks
  - OAuth token-based session auth
  - Artist image scraping from Last.fm web pages (OpenGraph)
  - Background artist info cache refresh (8-second interval)
- **ListenBrainz integration**:
  - Scrobble plays and now-playing updates
  - Custom server URL support (self-hosted ListenBrainz)
  - Token-based auth

### Jukebox Mode

- **MPV-based**: Uses MPV media player as backend via IPC (Unix socket)
- **Full control**: play, pause, stop, skip, seek, volume, playlist management
- **Extra args**: Pass custom MPV arguments (`-jukebox-mpv-extra-args`)
- **Requires MPV >= 0.34.0**
- **Supports**: tracks, podcast episodes, and internet radio stations in the jukebox playlist

### Podcast Management

- **RSS feed subscription**: Add podcasts by URL
- **Automatic refresh**: Hourly feed refresh for new episodes
- **Auto-download**: Per-podcast setting to auto-download latest episodes
- **Episode download queue**: Background download worker (5-second polling)
- **Episode purge**: Configurable age-based purge of old downloaded episodes (`-podcast-purge-age` days)
- **Cover art**: Automatic podcast cover image download
- **Tag reading**: Audio metadata extraction for downloaded episodes (bitrate, duration)

### Admin Web UI

HTML-based admin panel at `/admin/` with:

**Public routes**:
- Login page

**User routes** (any authenticated user):
- Home dashboard (library stats, recent folders, scan status, user list)
- Change username / password
- Change / delete user avatar
- Link / unlink Last.fm account
- Link / unlink ListenBrainz account (supports custom server URL)
- Create / delete transcode preferences per client

**Admin routes** (admin users only):
- Create new users
- Update Last.fm API key and secret
- Start incremental / full library scan
- Add / delete / update / download podcasts
- Add / delete / update internet radio stations

### Server & Deployment

- **Listen address**: Configurable bind address and port (default `0.0.0.0:4747`)
- **TLS support**: Optional TLS with cert/key files
- **Reverse proxy support**: Configurable path prefix (`-proxy-prefix`) for running behind a reverse proxy
- **HTTP logging**: Optional request logging
- **CORS**: Basic CORS headers enabled by default
- **Graceful shutdown**: Signal handling (SIGINT, SIGTERM) with clean shutdown of all goroutines
- **Debug endpoints**: Optional pprof (`-pprof`) and expvar (`-expvar`) endpoints
- **Database logging**: Optional GORM query logging (`-log-db`)
- **Config file support**: Load configuration from file (`-config-path`)
- **Environment variables**: All flags can be set via env vars (via `flagconf`)

### Background Jobs

The server runs several concurrent background goroutines managed by `errgroup`:

| Job                     | Interval     | Description                              |
|-------------------------|--------------|------------------------------------------|
| HTTP server             | continuous   | Main request handler                     |
| Scan watcher            | on-change    | Filesystem event-driven rescanning       |
| Scan timer              | configurable | Periodic full library scan               |
| Scan at start           | once         | Initial scan on startup                  |
| Jukebox                 | continuous   | MPV process management                   |
| Session cleanup         | 10 minutes   | Prune expired sessions from DB           |
| Podcast refresh         | 1 hour       | Refresh all podcast RSS feeds            |
| Podcast download        | 5 seconds    | Download queued podcast episodes         |
| Podcast purge           | 24 hours     | Remove old podcast episodes              |
| Transcode cache eject   | configurable | LRU eviction of transcoded audio cache   |
| Artist info refresh     | 8 seconds    | Background cache refresh from Last.fm    |

### Audio Format Support

Depends on the tag reader backend:
- **taglib**: All formats supported by TagLib (MP3, FLAC, OGG, Opus, AAC/M4A, WMA, WAV, AIFF, APE, WavPack, etc.)
- **ffprobe**: All formats supported by FFmpeg

### Data Integrity

- **Foreign key cascades**: All relationships use `ON DELETE CASCADE` for referential integrity
- **Transaction chunking**: Bulk deletes chunked to stay within SQLite's variable limit (999)
- **Atomic scanning flag**: `atomic.CompareAndSwap` prevents concurrent scans
- **Mutex-protected jukebox**: Read/write mutex on all MPV IPC operations
- **Mutex-protected playlists**: File operations serialized per store instance
