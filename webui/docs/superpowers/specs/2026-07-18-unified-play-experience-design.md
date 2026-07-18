# Unified Play Experience — Design

Date: 2026-07-18

## Goal

Give every documented **detail view** the same primary-playback affordance. The read-mode
actions — **Play**, **Add to queue**, and **Star** — move out of the `ContentScaffold`
action bar and into the `HeroHeader` below the identity, so playback lives with the artwork
and title on every screen.

This is the read-mode counterpart to the [Unified Edit Experience](../../../../docs/architecture/unified-edit-experience.md):
edit chrome stays in the scaffold bar; play/queue/star live in the hero.

- The **edit affordance** (`EditActionBar`: pencil → Save/Cancel/Delete) stays in the
  scaffold `#actions` bar, unchanged.
- The **album drag-to-queue handle** (`pi pi-bars`) stays in the scaffold bar for now,
  explicitly left untouched.
- Where a view is editable, its hero actions are **read-mode only** — hidden while editing,
  exactly as the scaffold's `EditActionBar` hides its own `#read-actions` in edit mode.

## Scope

Detail views under `PlayerLayout` that render a `HeroHeader`:

| View | Route | Hero actions | Star? | Queue? |
|------|-------|--------------|-------|--------|
| `AlbumView` | `/album/:id` | Play · Add to queue · Star | yes (album) | yes |
| `ArtistView` | `/artist/:id` | Play · Add to queue · Star | yes (artist) | yes |
| `PlaylistDetailView` | `/playlist/:id` | Play · Add to queue | no | yes |
| `RadioStationDetailView` | `/radio/:id` | Play | no | no |

**Applicability rules:**
- **Star** applies only where the entity is starrable in Subsonic — albums and artists.
  Playlists and radio stations are not starrable, so no Star.
- **Add to queue** applies to entities that expand to a track list — albums, artists,
  playlists. A radio station is a single live stream, so Radio is **Play only**.

**Out of scope:** all non-detail main content views (Library, Search, Radio list, Playlists
list, Genres, Home, Now Playing) — they have no hero. `RadioStationDetailView` create mode
(`/radio/new`) has no read mode, so no hero actions render there.

## Approach

A shared presentational component **`HeroActions`** renders the play/queue/star button set,
so their icons, styling, and spacing live in exactly one place — the play-side analogue of
`EditActionBar`. Each view composes it into a new **`#actions` slot on `HeroHeader`**, which
places it in the identity column and hides it in edit mode.

Rejected alternatives:
- *Bake the buttons into `HeroHeader` directly* — couples the generic identity/cover frame to
  playback semantics and the varying per-view action sets. The slot keeps `HeroHeader` generic,
  matching how `ContentScaffold` exposes a generic `#actions` slot.
- *Inline the buttons per view* — four copies of the Play/Queue/Star markup to keep in sync;
  the drift this change is meant to remove.

## Component: `HeroActions.vue`

Location: `webui/src/components/layout/HeroActions.vue`.

**Props**
- `playLabel?: string` (default `Play`).
- `playDisabled?: boolean` (default `false`) — disables Play (e.g. empty playlist).
- `busy?: boolean` (default `false`) — Play shows a loading spinner (used by Artist while it
  gathers songs).
- `canQueue?: boolean` (default `false`) — when `false`, Add to queue is not rendered.
- `canStar?: boolean` (default `false`) — when `false`, Star is not rendered.
- `starred?: boolean` (default `false`) — selects the filled/outline star icon.

**Emits**
- `play` — Play clicked.
- `queue` — Add to queue clicked.
- `star` — Star clicked.

**Structure**
- **Play**: filled Button, `icon="pi pi-play"`, `:label="playLabel"`, `:disabled="playDisabled"`,
  `:loading="busy"`, `@click` → `emit('play')`.
- **Add to queue** (when `canQueue`): Button `icon="pi pi-plus"`, `label="Add to queue"`,
  `severity="secondary"`, `text`, `@click` → `emit('queue')`.
- **Star** (when `canStar`): icon-only Button, `text rounded`,
  `:icon="starred ? 'pi pi-star-fill' : 'pi pi-star'"`, tooltip "Toggle star",
  `@click` → `emit('star')`.
- Buttons sit in a single `.hero-actions` row with a small gap.

## `HeroHeader.vue` change

Add an `#actions` slot in the `hero-info` column, below the read/edit identity blocks, wrapped
so it is **read-mode only**:

```vue
<div class="hero-actions-slot read-only"><slot name="actions" /></div>
```

The existing rule `.hero-header.editing .read-only { display: none }` already hides anything
tagged `read-only` in edit mode, so the actions disappear when the pencil flips the view into
edit mode with no extra logic. Purely additive — the `#read` and `#edit` slots are unchanged.

## Per-view changes

### AlbumView
- Remove Play, Add to Queue, and Star from the scaffold `#actions` (leave the drag-handle).
- Add `HeroActions` to `HeroHeader`'s `#actions`:
  - `:play-disabled="!album.song?.length"`, `can-queue`, `can-star`, `:starred="!!album.starred"`.
  - `@play="playAlbum"`, `@queue="addToQueue"`, `@star="handleStar"` (existing handlers).

### ArtistView
- Remove Star from the `EditActionBar` `#read-actions` (that slot becomes empty).
- Add `HeroActions` to `HeroHeader`'s `#actions`: `can-queue`, `can-star`,
  `:starred="!!artist.starred"`, `:busy="gathering"`, `@play`, `@queue`, `@star="handleStar"`.
- New `gatherSongs()`: fetch every album's songs via `subsonicClient.getAlbum(album.id)` in
  `sortedAlbums` order, flatten to a single `Song[]`. A `gathering` ref drives `busy` and
  guards against concurrent runs.
  - `@play` → gather, then `player.playAlbum(songs)`.
  - `@queue` → gather, then `player.addMultipleToQueue(songs)`.

### PlaylistDetailView
- Remove Play from the `EditActionBar` `#read-actions` (that slot becomes empty).
- Add `HeroActions` to `HeroHeader`'s `#actions`: `:play-disabled="working.length === 0"`,
  `can-queue`, `@play="playAll"`, `@queue` → `player.addMultipleToQueue(working)`.
- No star.

### RadioStationDetailView
- Remove Play from the `EditActionBar` `#read-actions` (that slot becomes empty).
- Add `HeroActions` to `HeroHeader`'s `#actions`: Play only (`can-queue`/`can-star` default
  false), `@play="onPlay"` (existing `playNow(stationToSong(station))`).

## Registry doc

New `docs/architecture/unified-play-experience.md` records the canonical behavior:
- The hero owns Play / Add-to-queue / Star in **read mode**; they hide in edit mode.
- `HeroActions` and the `HeroHeader` `#actions` slot are the single sources of truth.
- The per-view applicability table (star = starrable entities only; queue = track-list
  entities only; radio = Play only).
- Drag-to-queue is explicitly left in the scaffold bar for now.

Cross-link it from `docs/architecture/main-content-view-layout.md` (in the reference-implementations
list and the conformance section), mirroring how `unified-edit-experience.md` is linked.

## Testing

- **`HeroActions.spec.ts`** (new): Play always renders and emits `play`; Add-to-queue renders
  only when `canQueue` and emits `queue`; Star renders only when `canStar`, emits `star`, and
  its icon reflects `starred`; `playDisabled` disables Play; `busy` puts Play in loading state.
- **AlbumView.spec.ts**: Play/Queue/Star now live in the hero, not the scaffold bar; handlers
  still fire; drag-handle still in the scaffold.
- **ArtistView.spec.ts**: hero Star present and emits; Play gathers album songs then calls
  `playAlbum`; Queue gathers then calls `addMultipleToQueue`; actions hidden in edit mode.
- **PlaylistDetailView.spec.ts**: hero Play + Queue present; Play disabled when empty; actions
  hidden in edit mode; edit-mode button set unchanged.
- **RadioStationDetailView.spec.ts**: hero shows Play only; Play calls the play handler; no
  hero actions in create mode; edit-mode set unchanged.
