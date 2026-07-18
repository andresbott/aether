# Playlist UI Rework — Design

**Date:** 2026-07-15
**Status:** Approved for planning

## Goal

Rework the playlist UI so that:

1. Playlist track editing behaves like the queue edit mode and **reuses the same
   component**.
2. The playlist rename control sits **inline beside the playlist title**.
3. The playlists list view mirrors the **albums view**: click a card to enter, a
   hover play button, and a grid/list layout toggle.

## Context

- Frontend: Vue 3 + PrimeVue + TanStack Query. Main content views follow the
  `ContentScaffold` layout (`docs/architecture/main-content-view-layout.md`).
- The queue edit mode (`QueueView.vue`) already implements a multi-select,
  drag-to-reorder, delete-selected track list backed by SortableJS and
  `useRowSelection`. It operates entirely in memory against `usePlayer()` state.
- The Subsonic backend (this branch) persists a playlist reorder by **replacing
  the whole ordered track list** in one call: `createPlaylist.view` with an
  existing `playlistId` plus the full `songId` set (`store.SetPlaylistTracks`).
  This is the standard Subsonic mechanism and maps cleanly to a batched edit.

## Decisions (from brainstorming)

- **Edit model:** Batched. Enter edit mode → reorder/delete on a local working
  copy with instant feedback → **Save** commits the whole new order in one
  replace-all call; **Cancel** discards.
- **Rename:** Inline edit in place. Pencil beside the title swaps the title text
  for an input; Enter/✓ saves, Escape/✕ cancels. No dialog.
- **Add tracks in edit mode:** Out of scope. Edit mode = reorder + delete only.
- **Playlists layout default:** Grid (like the Library/albums view).

---

## Part 1 — Shared reorderable track-edit list

Extract the queue's edit-mode list into a new reusable component
`src/components/layout/TrackEditList.vue`. This is the "same component" the queue
and playlists both use.

**Responsibilities (owned internally):**

- `useRowSelection` — plain / ctrl-⌘ / shift multi-select.
- SortableJS wiring: `handleSortStart`, `setData`, `handleSortEnd`, teardown, and
  the stacked multi-drag image (`buildMultiDragImage`).
- Focusable listbox with Delete/Backspace acting on the current selection.
- Renders `QueueRow` in `editing` mode for each row.

**Interface:**

```
props:
  songs: Song[]              // the ordered list to edit
  currentIndex?: number      // now-playing marker; omitted for playlists
  deleteLabel?: string       // row delete tooltip; default "Remove"
emits:
  reorder(indices: number[], target: number)
  delete(indices: number[])
```

The component **does not persist anything**. It emits intent; the parent decides
what `reorder`/`delete` mean. `reorder` reports the selection being moved
(`selectionForDrag`) and the computed target index (`computeDropTarget`), exactly
as the queue does today.

**`QueueRow.vue`** stays as-is but gains an optional `deleteLabel` prop (it
currently hard-codes the "Delete from queue" tooltip). It is already generic over
`{ song, queueIndex }`.

**`QueueView.vue`** after extraction:

- Edit branch becomes:
  `<TrackEditList :songs="player.queue.value" :current-index="player.currentIndex.value" delete-label="Delete from queue" @reorder="(idx, t) => player.moveInQueue(idx, t)" @delete="removeIndices" />`
- Keeps: the `editMode` toggle in its header, the drop-to-add logic
  (`useQueueDrop` on `queueBodyRef`), and the view-mode history / now-playing /
  upcoming rendering.
- `useQueueEdit` collapses to just the `editMode` boolean (selection state now
  lives inside `TrackEditList`).

---

## Part 2 — `PlaylistDetailView` rework

**Inline rename**

- Add an optional `#title-actions` slot to `ContentScaffold.vue`, rendered inside
  `.scaffold-title` after the `<h1>`.
- The detail view fills it with a small pencil button. Clicking it swaps the
  title for an `InputText` in place. Enter or ✓ saves via
  `useUpdatePlaylist({ name })`; Escape or ✕ cancels. Empty name is rejected
  (no-op).

**Edit mode (batched)**

- A `pi pi-pencil` toggle in `#actions` (queue parity) enters edit mode.
- On enter: clone `playlist.entry` into a local `working` ref.
- `TrackEditList` renders `working`. `reorder(indices, target)` and
  `delete(indices)` mutate the local array only — instant feedback, no server
  call.
- While in edit mode the actions area shows **Save** + **Cancel** instead of the
  normal actions.
  - **Save:** `useReplacePlaylistTracks({ playlistId, songIds: working.map(s => s.id) })`,
    then exit edit mode. Shows loading state on the button.
  - **Cancel:** discard `working`, exit edit mode.
- View mode is otherwise unchanged (Play + delete-playlist actions + the edit
  toggle). Reworking the view-mode hero/table is out of scope.

**Note on two pencils:** the detail view now shows a pencil inline by the title
(rename) and a pencil in the actions (edit tracks, matching the queue). Different
locations and scopes. If ambiguous in practice, the edit-tracks toggle can move
to a different icon (`pi pi-list` / `pi pi-sort-alt`) — resolve during
implementation review.

---

## Part 3 — `PlaylistsView` grid/list (albums-style)

- **`src/components/library/PlaylistCard.vue`** (mirrors `AlbumCard.vue`): cover,
  name, `songCount` subtitle, hover play button, whole card is a `router-link` to
  `playlist-detail`. Play fetches `getPlaylist(id)` entries then
  `player.playAlbum(entries)` (album summaries carry no tracks; same pattern as
  `AlbumCard`).
- **`src/components/library/PlaylistListView.vue`** (mirrors `AlbumListView`'s row
  layout but simpler — no virtualization or alphabet rail, since `getPlaylists`
  returns the full set at once): rows with cover, name, count, play button;
  click-to-open.
- **`PlaylistsView.vue`**: add the `SelectButton` grid/list toggle (route-query
  driven, like `LibraryView`), default **grid**. Keep the Create Playlist action
  and dialog. Renders `PlaylistListView` when `layout === 'list'`, otherwise a
  grid of `PlaylistCard`.

---

## Part 4 — API / persistence

- Add `subsonicClient.replacePlaylistTracks(playlistId: string, songIds: string[])`
  calling `createPlaylist.view?playlistId=…&songId=…` (the backend replace-all
  path). Reuses the existing response/error handling.
- Add `useReplacePlaylistTracks()` mutation hook that invalidates
  `queryKeys.playlists` and `['subsonic', 'playlist']`, consistent with the other
  playlist mutations.

---

## Part 5 — Testing

- **`TrackEditList.spec.ts`**: multi-select (plain/ctrl/shift), `reorder` emit
  reports correct indices + target, `delete` emit, keyboard Delete/Backspace on
  the focused list.
- **`QueueView.spec.ts`**: update to the extracted component; assert emits are
  wired to `player.moveInQueue` / remove.
- **`PlaylistDetailView.spec.ts`**: enter edit → local reorder/delete leave the
  server untouched → Save calls `replacePlaylistTracks` with the working order →
  exits edit mode; Cancel discards; inline rename saves/cancels.
- **`PlaylistCard.spec.ts`** / **`PlaylistListView.spec.ts`**: navigates to
  detail on click; play fetches entries and calls `player.playAlbum`.
- **`PlaylistsView.spec.ts`** (if present / added): layout toggle switches
  grid ↔ list and persists to the route query.

## Out of scope

- Adding tracks to a playlist via drag/drop in edit mode.
- Reworking the playlist detail view-mode hero/track table.
- Playlist public/private toggle UI (backend supports it; not requested here).
