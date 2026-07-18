# Unified Play Experience — Registry

Canonical behavior for the **read-mode playback actions** on detail views. Read this before
adding or changing a Play / Add-to-queue / Star affordance. This is the play-side counterpart
to [`unified-edit-experience.md`](unified-edit-experience.md): edit chrome lives in the
`ContentScaffold` action bar; **playback lives in the hero**.

## The rule

- **Play, Add to queue, and Star live in the `HeroHeader`**, below the identity, rendered by
  the shared **`HeroActions`** component (`webui/src/components/layout/HeroActions.vue`).
- They are **read-mode only**. `HeroHeader` renders its `#actions` slot behind `v-if="!editing"`,
  so entering edit mode (the scaffold pencil) removes them; Save/Cancel/Delete then own the bar.
- The `ContentScaffold` `#actions` bar carries **only** edit chrome (`EditActionBar`) and, on
  AlbumView, the drag-to-queue handle. No Play/Queue/Star buttons in the scaffold bar.

## Single sources of truth

- **`HeroActions.vue`** — the button set, icons, styling, and emits. Props: `playLabel`,
  `playDisabled`, `busy`, `canQueue`, `canStar`, `starred`. Emits: `play`, `queue`, `star`.
  Stable classes: `.hero-action-play`, `.hero-action-queue`, `.hero-action-star`.
- **`HeroHeader` `#actions` slot** — the placement + the read-mode `v-if` gate.

## Applicability per view

| View | Play | Add to queue | Star | Notes |
|------|------|--------------|------|-------|
| `AlbumView` | ✅ | ✅ | ✅ | Songs already loaded; drag handle stays in the scaffold bar |
| `ArtistView` | ✅ | ✅ | ✅ | `getArtist` has no songs — Play/Queue gather via `getAlbum` per album (`busy` while gathering) |
| `PlaylistDetailView` | ✅ | ✅ | — | Uses the live `working` track list; Subsonic does not star playlists |
| `RadioStationDetailView` | ✅ | — | — | Single live stream: queue/star not applicable |

**Applicability rules:** Star only where the entity is starrable (albums, artists). Add-to-queue
only where the entity expands to a track list (albums, artists, playlists).

## Deliberately out of scope

- **Drag-to-queue** (the album `pi pi-bars` handle) stays in the scaffold bar for now — not
  moved into the hero.
- Non-detail main content views (Library, Search, Radio/Playlists lists, Genres, Home, Now
  Playing) have no hero and are unaffected. Radio **create** mode (`/radio/new`) has no read
  mode, so no hero actions render there.
