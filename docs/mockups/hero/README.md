# Album detail-view redesign — implementation handoff

**Status:** design approved (mock only, not wired into the app). No code changed yet.

## The mock
`docs/mockups/hero/` — the chosen design ("07 Duotone" hero) as a 3-state flow of
static HTML/CSS:

| File | State |
|------|-------|
| `07-duotone-select.html` | Track rows multi-selected; hero action row = selection controls |
| `07-duotone-add-to-playlist.html` | The "which playlist?" picker open |
| `07-duotone-added.html` | Selection cleared, success toast |

Preview: `cd docs/mockups/hero && python3 -m http.server`, open the files, toggle
light/dark bottom-right. Shared deps: `_shared.css`/`_shared.js` (tokens + track
list + player bar), `_select.css`/`_select.js` (selection layer), `cover.svg`
(sample art). `_select.css` has dead `.sel-bar` rules from an earlier version.

## What to build
1. **Edit moves into the hero.** Today it's `EditActionBar` in the top bar; the top
   bar keeps only **Back**, Edit lives in the hero.
2. **Hero = "07 Duotone":** cover art desaturated + re-inked cyan→magenta behind the
   sharp cover, dark scrim, light ink. Holds type · name · meta ·
   Play/Add-to-queue/Favorite · Edit. Visually distinct from the track-list body.
3. **In-hero multi-select:** while rows are selected, the hero's own action row
   *retargets* to the selection (frosted strip: Play · Add to queue · Add to
   playlist · Favorite · Clear). No floating bar near the player.
4. **Playlist picker** anchored under the in-hero "Add to playlist" button — search,
   playlist covers + song counts, pinned "New playlist". Confirm with a toast.
5. Generalize to the other four detail views (adaptations below).

## Files to change (`webui/src/`)
| Concern | Path |
|---------|------|
| Hero component (restyle, receive Edit) | `components/layout/HeroHeader.vue` (+ `HeroActions.vue`) |
| Top bar — remove Edit from `#actions` | `components/layout/ContentScaffold.vue` |
| Edit control — relocate out of the top bar | `components/layout/EditActionBar.vue` |
| Detail views (5) that compose the hero | `views/AlbumView.vue`, `views/ArtistView.vue`, `views/PlaylistDetailView.vue`, `views/RadioStationDetailView.vue`, `views/GenreDetailView.vue` |
| Track row + row selection | `components/library/AlbumTrackRow.vue`, `composables/useRowSelection.ts` |
| Per-track menu — vocabulary/behaviour to mirror | `components/library/TrackActionSheet.vue` |
| Playlists data + add mutation | `composables/useSubsonicQueries.ts` → `usePlaylists`, `useUpdatePlaylist({ playlistId, songIdsToAdd })` |
| Design tokens (the mock mirrors these) | `assets/scss/_variables.scss` |

## Per-view adaptation
- **Artist** — round cover, no year; meta "N albums · M songs"; Play/Shuffle/Favorite/Edit.
- **Playlist** — description + "by {owner}"; Edit = name + description + cover.
- **Radio** — broadcast mark for cover; meta = stream / homepage URL; **Play + Edit only** (no queue/favorite).
- **Genre** — tag-style placeholder cover; meta "N songs · M albums"; Play/Add-to-queue/Edit.

## Key facts / constraints
- **Palette/type:** accent `--app-accent` (`#0e9bb5` light / `#2fd3ef` dark), magenta
  `--app-nav-brand-alt` `#d81b60`, Inter, 8px radius — everything via `--app-*` tokens.
- **Actions vocabulary (match `TrackActionSheet.vue`):** Play · Add to queue · Add to
  favorites · Add to playlist → pick a playlist → toast "Added to {name}".
- **New work:** the bulk selection action row and a **desktop playlist picker don't
  exist yet** — today `useRowSelection` only feeds drag-to-queue/playlist.
- **Boundaries:** all music features stay on `/rest` (OpenSubsonic); playlists via
  `getPlaylists` / `updatePlaylist`. Follow `docs/architecture/main-content-view-layout.md`
  (uniform main-content layout). No backwards-compat needed (drop/replace freely).
