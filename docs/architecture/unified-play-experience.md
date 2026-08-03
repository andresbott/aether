# Unified Play Experience — Registry

Canonical behavior for the **read-mode playback actions** on detail views. Read this before
adding or changing a Play / Add-to-queue / Favorite affordance. This is the play-side counterpart
to [`unified-edit-experience.md`](unified-edit-experience.md): edit chrome lives in the
`ContentScaffold` action bar; **playback lives in the hero**.

## The rule

- **Play, Add to queue, and Favorite live in the `HeroHeader`**, below the identity, rendered by
  the shared **`HeroActions`** component (`webui/src/components/layout/HeroActions.vue`).
- They are **read-mode only**. `HeroHeader` renders its `#actions` slot behind `v-if="!editing"`,
  so entering edit mode (the scaffold pencil) removes them; Save/Cancel/Delete then own the bar.
- The `ContentScaffold` `#actions` bar carries **only** edit chrome (`EditActionBar`) and, on
  AlbumView, the drag-to-queue handle. No Play/Queue/Favorite buttons in the scaffold bar.

## Single sources of truth

- **`HeroActions.vue`** — the button set, icons, styling, and emits. Props: `playLabel`,
  `playDisabled`, `busy`, `canQueue`, `canStar`, `starred`. Emits: `play`, `queue`, `star`.
  Stable classes: `.hero-action-play`, `.hero-action-queue`, `.hero-action-star`.
  The props/emits keep the Subsonic wire vocabulary (`starred`, `star`); the **visible**
  affordance is a favorite — `pi pi-heart` / `pi pi-heart-fill` with an
  "Add to favorites" / "Remove from favorites" tooltip and `aria-label`. Every favorite
  toggle in the app uses that same icon pair and wording (`AlbumCard`, `ArtistCard`,
  `PlaylistCard`, `PlaylistListView`, `SongDetail`, `PlayerControls`, the Discovery
  *Favorites* section icon) — do not introduce a star glyph or "Like"/"Star" label.
  The button is `severity="secondary"`: without one it rendered in the primary accent
  in **both** states, saying nothing about the state.
- **A favorite is signalled by the FILL, not by colour.** `pi-heart-fill` in the
  same grey as the outline state (`--app-text-secondary`, hover
  `--app-text-primary`) — no accent, no `danger` red. The accent has two other jobs
  (what is playing, what is actionable) and a list of accent hearts competed with
  both. **One deliberate exception:** `PlayerControls`' `.now-like.liked` keeps
  `--app-accent`, because it is a single heart on dark player chrome rather than one
  of a list, so nothing competes and the accent is what makes it findable. Don't
  "unify" that one away, and don't reintroduce colour anywhere else.
- **Grid cards** (`AlbumCard`, `ArtistCard`, `PlaylistCard`) share one `.card-star`
  pattern: `opacity: 0` by default, revealed by `.<entity>-card:hover`, and pinned
  visible via `.is-starred` so a grid reads as a set of favorites at a glance.
  `.is-starred` now controls **visibility only** — the fill carries the state. The cards
  are `router-link`s, so the handler must `preventDefault()` **and**
  `stopPropagation()` or the click navigates. Copy an existing card rather than
  inventing a fourth variant.
- **Track rows** render **`TrackFavoriteButton`** (`components/library/`) — one
  component, not a per-row copy of the card pattern. It owns the icon pair, the
  wording, the `.row-star`/`.is-starred` classes and the click swallowing (both
  `click` and `dblclick`: rows select on one and play on the other). Each host row
  supplies only its own reveal rule (`.<row>:hover .row-star { opacity: 1 }`),
  because the hover selector is row-specific while nothing else is. Hosts:
  `AlbumTrackRow`, `GenreTrackRow` (genre detail, search, playlist detail) and
  `QueueRow` in view mode. Edit mode gets no heart — that mode is for reordering
  and removal.
- **Entity rows** (`AlbumRow`, `ArtistRow`, `PlaylistRow`) each own an inline
  `.row-star` button rather than `TrackFavoriteButton`: they take an album/artist/
  playlist, not a `Song`, and they need no optimistic flip — all three are
  query-backed, so `useToggleStar`'s invalidation refreshes the heart. Same look and
  wording, and the same swallow-the-click requirement (they are `router-link`s).
  Playlists use `useTogglePlaylistStar` (the `playlistStar` extension), the other
  two `useToggleStar`.
- **The favorite column is a `2rem` grid track** in every list, and the row's
  template must be mirrored by its list header or the columns drift apart. Four
  pairs to keep in step: `AlbumRow` + `PlaylistRow` (identical
  `48px 2fr 1.5fr 2rem 4rem 5rem`, so a Discovery feed interleaving them aligns)
  against `AlbumListView`'s and `PlaylistListView`'s headers plus `DiscoveryFeed`'s
  list header; and `ArtistRow` (`48px 1fr 2rem 7rem`) against `ArtistListView`'s.
  A header cell for the star column stays **blank** — labelling a hover-revealed
  control teaches nothing. Cells that a row might otherwise omit have to render
  empty rather than `v-if` away (see `ArtistRow`'s album count), or the heart slides
  into the next column.
- **Entity rows carry no play button.** `PlaylistRow` had one and it was removed:
  `AlbumRow` and `ArtistRow` have none, the row itself opens the entity, and play
  lives on the grid cards and the detail hero.
- **`useSongFavorite(song)`** — the one place a song's `starred` is read and
  written, behind every heart above plus `SongDetail` and (via
  `useCurrentTrackFavorite`) the player bar and the `L` shortcut. It flips
  `song.starred` locally as well as mutating, because the play queue is plain
  reactive state with no query to refetch. **That means a row must be handed the
  actual song object** — `QueueBody` passes `{ song, queueIndex }` rather than
  spreading the song, or the optimistic write would land on a throwaway copy and
  the heart would snap back.
- **`HeroHeader` `#actions` slot** — the placement + the read-mode `v-if` gate.

## Applicability per view

| View | Play | Add to queue | Favorite | Notes |
|------|------|--------------|----------|-------|
| `AlbumView` | ✅ | ✅ | ✅ | Songs already loaded; drag handle stays in the scaffold bar |
| `ArtistView` | ✅ | ✅ | ✅ | `getArtist` has no songs — Play/Queue gather via `getAlbum` per album (`busy` while gathering) |
| `PlaylistDetailView` | ✅ | ✅ | ✅ | Uses the live `working` track list; playlist favorites come from the `playlistStar` extension |
| `RadioStationDetailView` | ✅ | — | — | Single live stream: queue/favorite not applicable |

**Applicability rules:** Favorite only where the entity is starrable — albums, artists,
songs and playlists. Genres and radio stations are **not** starrable in the OpenSubsonic
standard or in any Aether extension, so they get no toggle (see
[`../agents/subsonic-api.md`](../agents/subsonic-api.md), "Favorites"). Add-to-queue only
where the entity expands to a track list (albums, artists, playlists).

A hero Favorite and a track-row Favorite are **different entities** on the same view:
AlbumView's hero star is the album's, its row hearts are each track's. A view listing
songs therefore has row hearts regardless of whether it has a hero — Search and the
queue have no hero and still star their tracks.

## Deliberately out of scope

- **Drag-to-queue** (the album `pi pi-bars` handle) stays in the scaffold bar for now — not
  moved into the hero.
- Non-detail main content views (Library, Search, Radio/Playlists lists, Genres, Home, Now
  Playing) have no hero and are unaffected. Radio **create** mode (`/radio/new`) has no read
  mode, so no hero actions render there.
