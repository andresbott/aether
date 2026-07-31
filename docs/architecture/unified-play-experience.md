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
- **Grid cards** (`AlbumCard`, `ArtistCard`, `PlaylistCard`) share one `.card-star`
  pattern: `opacity: 0` by default, revealed by `.<entity>-card:hover`, and pinned
  visible via `.is-starred` so a grid reads as a set of favorites at a glance. The cards
  are `router-link`s, so the handler must `preventDefault()` **and**
  `stopPropagation()` or the click navigates. Copy an existing card rather than
  inventing a fourth variant.
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

## Deliberately out of scope

- **Drag-to-queue** (the album `pi pi-bars` handle) stays in the scaffold bar for now — not
  moved into the hero.
- Non-detail main content views (Library, Search, Radio/Playlists lists, Genres, Home, Now
  Playing) have no hero and are unaffected. Radio **create** mode (`/radio/new`) has no read
  mode, so no hero actions render there.
