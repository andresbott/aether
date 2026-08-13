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
- **Grid-card controls are always visible, dimmed when the card is not hovered.**
  Both `.card-star` and `.card-play` sit at `opacity: 0.4` by default and go to `1`
  on `.<entity>-card:hover`. They were previously `opacity: 0` (hover-only) — a card
  whose actions appear only on hover doesn't advertise that it has any, and on touch
  there is no hover at all. Applies to `AlbumCard`, `ArtistCard`, `PlaylistCard` and
  `RadioStationCard` (play only).
- **A dimmed heart is dimmed whether or not it is a favorite.** There is no
  `.card-star.is-starred { opacity: 1 }` pin any more: the **fill** already tells the
  two apart at 0.4, and pinning favorites bright made a grid of favorites look like a
  grid with a broken hover state. `.is-starred` remains on the element for tests and
  for anything that needs the state, but it no longer affects opacity — the fill is
  the whole signal, consistent with the colour rule above. Rows still pin
  (`.row-star.is-starred`), because there the alternative is invisible, not dim.
  The cards are `router-link`s, so the handler must `preventDefault()` **and**
  `stopPropagation()` or the click navigates. Copy an existing card rather than
  inventing a new variant.
- **Rows keep the hover-only reveal.** A list is dense enough that a column of
  permanently-visible hearts and the row text compete; grids have the whitespace
  for it, lists don't. `TrackFavoriteButton` / `.row-star` are unchanged.
- **Double-clicking a track row APPENDS it to the end of the queue** — it does not
  replace the queue. The row emits **`enqueue`** (not `play`), and every host answers
  it with `player.enqueueAndPlayIfIdle([song])`: the track lands last, and playback
  starts only when the player was idle (empty queue or no loaded track), so a
  double-click on an idle player still makes sound while one on a playing queue never
  interrupts it. Hosts: `AlbumView`, `GenreDetailView`, `SearchView`,
  `PlaylistDetailView`. **Replacing the queue belongs to the hero Play button**
  (`player.playAlbum`) **and, on touch only, to a row tap** (see "Touch contract") —
  do not wire a row double-click back to `playAlbum`, and do not
  add a per-view variant of the gesture: it means the same thing on every track list.
  `QueueRow` is the exception and keeps `play` — inside the queue, double-clicking a
  row means "jump to this slot" (`playQueueItem`), since the track is already queued.
- **Track rows** render **`TrackFavoriteButton`** (`components/library/`) — one
  component, not a per-row copy of the card pattern. It owns the icon pair, the
  wording, the `.row-star`/`.is-starred` classes and the click swallowing (both
  `click` and `dblclick`: rows select on one and enqueue on the other). Each host row
  supplies only its own reveal rules (`.<row>:hover .row-star { opacity: 1 }` plus the
  `@media (pointer: coarse)` unconditional reveal, since touch has no hover),
  because those selectors are row-specific while nothing else is. All three rows
  carry both — a row with only the hover rule has an invisible heart on touch. Hosts:
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
- **Two hearts in the app are FILTERS, not toggles:** `LibraryView`'s
  `.library-favorites-filter` (narrows the Albums/Artists tabs) and
  `PlaylistsView`'s `.playlists-favorites-filter` (narrows the playlist list).
  Both borrow the icon pair (`pi-heart` / `pi-heart-fill`) because they are about
  favorites, but their labels say what clicking does to the **list** — "Show
  favorites only" / "Show all" — never "Add to/Remove from favorites". Keep that
  distinction if you add a third: a heart that changes *what you see* must not be
  worded like one that changes *what is starred*. Both follow the colour rule
  (grey, fill-only signal), which meant overriding PrimeVue's checked state — a
  `ToggleButton` otherwise comes up in the primary accent — and both hide the
  empty label's `&nbsp;` span so they stay icon-width. The two CSS blocks are
  deliberate twins: change one, change both.
  They share a UX contract — `?favorites=1` in the URL beside `?view=list`, first
  in the scaffold action bar, a `"N favorites"` summary, and an empty state that
  says "No favorite X yet" rather than claiming none exist — but **not** a
  mechanism, and the difference is instructive. Library needs a whole second data
  source (`useLibrarySource`, hitting `getStarred2`) because its lists are paged
  and server-ordered; Playlists is a one-line client-side predicate on
  `pl.starred`, because `getPlaylists` is unpaginated and already carries the
  `starred` timestamp per row. Don't "unify" the playlist one onto `getStarred2` —
  it would be the same rows at the cost of an extra request.

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

## Touch contract

On `useViewport().isTouch`, a track-row tap **plays the view's visible track list
starting at the tapped track**: hosts wire `@play` → `player.playAlbum(list, index)`,
the same primitive the hero Play uses. Say it plainly — **a tap REPLACES the queue**.
That is the point: tapping track 3 of an album queues the whole album and starts at
3. It must **not** be `player.playNow(song)`, which sets `queue = [song]` and so
threw away the rest of the album plus whatever was already queued (and, on a
touch-capable desktop, did that on one stray mouse click).

Each host passes its own full list — `AlbumView` `orderedSongs` (flat, disc-ordered,
so the start index is the row's flat position), `PlaylistDetailView` `working` (the
live list, so an unsaved reorder plays as shown), `SearchView` `songs`, and
`GenreDetailView` the loaded entries of its **sparse** `items`. The genre table is
lazily paged, so unscrolled slots are holes and the queue must be dense: it queues
the pages already loaded and starts at the tapped song's position in that dense
list, which keeps the song under the finger the one that plays. Gathering the
complete genre stays the hero Play's job.

The per-row ⋮ (aria-label "Track actions") opens the shared **`TrackActionSheet`**
(`webui/src/components/library/TrackActionSheet.vue`) — add to queue via
`player.addToQueue`, favorite via `useSongFavorite`, add to playlist via
`updatePlaylist` `songIdsToAdd`, go to album, and go to artist. The playlist picker's
`getPlaylists` is **gated on first open** (`usePlaylists({ enabled })`): the sheet is
mounted by every host view, and an ungated query cost a request per visit for a
picker desktop users can never open. Add-to-playlist toasts both ways
(success + `apiErrorMessage`) — it is a write with no visible result otherwise.

Queue-replacing affordances are therefore the **hero Play** (both input kinds) and a
**row tap** (touch only). Double-click-to-enqueue and hover reveals remain the
**pointer contract** (additive, not replaced) — touch has no hover, so the heart is
visible always, via a `@media (pointer: coarse) { .row-star { opacity: 1 } }` rule in
each host row: `AlbumTrackRow`, `GenreTrackRow` **and `QueueRow`**. `QueueRow`
deliberately gets that rule and nothing else — no ⋮ and no `TrackActionSheet` in the
queue, where the sheet's actions are either meaningless (add to queue) or already
present. **`TrackSelectButton` is pointer-only** (`v-if="!isTouch"`), so multi-select
has no touch equivalent — the ⋮ takes its column on touch.

## Deliberately out of scope

- **Drag-to-queue** (the album `pi pi-bars` handle) stays in the scaffold bar for now — not
  moved into the hero.
- Non-detail main content views (Library, Search, Radio/Playlists lists, Genres, Home, Now
  Playing) have no hero and are unaffected. Radio **create** mode (`/radio/new`) has no read
  mode, so no hero actions render there. The Library and Playlists favorites *filters* live in
  the scaffold action bar rather than a hero — they are view state, not playback actions, and
  the "scaffold bar carries only edit chrome" rule is about Play/Queue/Favorite **toggles**.
