# Radio station CRUD in the `/radio` view

## Goal

Move internet-radio-station management out of Settings and into the main Radio
view (`/radio`). Discovering and adding stations happens from the view header;
editing and deleting a station happens in a dedicated detail view, mirroring the
playlist pattern (`PlaylistsView` → `PlaylistDetailView`).

All station CRUD continues to run over the existing Subsonic (`/rest/`) endpoints
via the existing query/mutation hooks; no API changes.

## Current state (for reference)

- `/radio` (`RadioView.vue`): browse only. Header has a grid/list `SelectButton`.
  Cards (`RadioStationCard`) and rows (`RadioStationRow`) are drag-to-queue with a
  hover play button; neither navigates.
- `/settings/radio` (`RadioStationsView.vue`): all CRUD lives here as a
  master-detail splitter (`StationList` + `StationEditPanel`) plus a "Search
  Online" (`StationSearchDialog`) and an "Add Station" button.
- `components/library/RadioStationDialog.vue`: an orphaned, unused add/edit dialog
  (self-labelled "Deprecated").
- Hooks (`composables/useSubsonicQueries.ts`): `useRadioStations` (list),
  `useCreateRadioStation`, `useUpdateRadioStation`, `useDeleteRadioStation`.
  There is no single-station GET — the Subsonic API only lists.

## Routes

In `router/index.ts`, add two routes rendering the new `RadioStationDetailView`,
both with `meta: { flush: true }`:

- `/radio/new` — name `radio-station-new`, `props: { create: true }`.
- `/radio/:id` — name `radio-station-detail`, `props: true`.

`/radio/new` MUST be declared **before** `/radio/:id` so the literal path wins
over the `:id` param.

Remove the `settings-radio` child route.

## `RadioView` header (`/radio`)

Keep the grid/list `SelectButton`. Add two controls to `#actions`, ordered
`SelectButton`, Discover, Add (matching `PlaylistsView`'s toggle + create button):

- **Discover** — `Button` `pi pi-globe`, outlined. Opens `StationSearchDialog`
  (`v-model:visible`).
- **Add Station** — `Button` `pi pi-plus`. `router.push({ name: 'radio-station-new' })`.

`StationSearchDialog` moves from `views/settings/radio-stations/` to
`components/library/` (its `RadioBrowserStation` import path and behavior are
unchanged). On `@select`, `RadioView` navigates to the create route carrying the
station's text fields and favicon URL as query params:

```
router.push({
  name: 'radio-station-new',
  query: { name, streamUrl, homepage, favicon }
})
```

Only defined values are added to the query (omit empty `homepage`/`favicon`).

## Discover prefill: carrying data through navigation

A route cannot carry a `File`, so Discover passes only text + the favicon **URL**.
`RadioStationDetailView` in create mode seeds the form fields from `route.query`
and, when `favicon` is present, calls `fetchRadioFavicon(faviconUrl)` in the
background and folds the resulting `File` into the form as the cover once ready.
The fetch is non-blocking and guarded so a slow/failed favicon never blocks the
form (same behavior the settings flow has today).

Rejected alternative: a shared module-level "prefill" ref to pass the `File`
object directly. Rejected as hidden mutable state; the query approach is
deep-linkable and straightforward to test.

## `RadioStationDetailView.vue` (new)

Layout copied from `PlaylistDetailView`: a back-row (`pi pi-arrow-left`,
`router.back()`), then `ContentScaffold` with `loading` / `error` / content
states.

**Create mode** (`create` prop true):
- Title: "Add station". No data fetch.
- Form seeded from `route.query`; favicon fetched as described above.
- Header action: **Create** button, disabled until the form is valid.
- On success: `router.push({ name: 'radio' })` (create returns no id, so return to
  the list).
- No Delete button.

**Edit mode** (`:id` param):
- Resolve the station from the cached `useRadioStations()` list by id. While the
  list is loading show the loading state; if the list has loaded and the id is not
  found, show the error/not-found state.
- Title: station name.
- Header actions: **Play** (`pi pi-play`, enqueues via `stationToSong`), **Save**
  (disabled until valid), **Delete** (`pi pi-trash`, danger, guarded by
  `ConfirmDialog`).
- Save success: stay on the page; the invalidated `radioStations` query refetches
  and re-seeds the form.
- Delete success: `router.push({ name: 'radio' })`.

## `RadioStationForm.vue` (new)

A presentational form extracted from the current `StationEditPanel` internals:
name, stream URL, homepage URL, and cover management (choose image, 5 MB size
guard with error message, live preview, remove/clear existing cover via
`coverClear`). It carries none of `StationEditPanel`'s splitter chrome (the
`h3` heading, the idle "select a station" state, or its own Save/Delete buttons).

Interface:
- Props: `station: InternetRadioStation | null`, `prefill?: RadioStationPrefill | null`.
- Emits: `change({ input, valid })` on every field/cover change, where `input` is
  `{ name, streamUrl, homepageUrl?, coverFile?, coverClear? }` and `valid` is true
  when `name` and `streamUrl` are non-empty and there is no size error.

The detail view owns the Create/Save/Delete buttons (in the scaffold header) and
holds the latest emitted `{ input, valid }`; its action buttons call the
create/update mutations with `input`.

**Unsaved-changes guard** (parity with `PlaylistDetailView`): the detail view
tracks a `dirty` flag (form differs from the seeded values, or a new cover file
was chosen, or cover-clear is set) and guards navigation with `onBeforeRouteLeave`
(`window.confirm`) and a `beforeunload` listener.

## Card / row navigation

- `RadioStationCard.vue`: wrap the card in
  `<router-link :to="{ name: 'radio-station-detail', params: { id: station.id } }">`
  (as `PlaylistCard` does). Preserve drag-to-queue (`draggable` + `dragstart`) and
  the hover play button; the play button's handler keeps `event.stopPropagation()`
  / `preventDefault()` so clicking play does not navigate.
- `RadioStationRow.vue`: same — the whole row becomes the navigating link,
  preserving drag and hover.

## Migration cleanup (deletions)

- `views/settings/RadioStationsView.vue`
- `views/settings/radio-stations/StationList.vue` + spec
- `views/settings/radio-stations/StationEditPanel.vue` + spec
- `views/settings/radio-stations/__tests__/RadioStationsView.spec.ts`
- `components/library/RadioStationDialog.vue` + spec (orphaned/deprecated)
- The `settings-radio` route in `router/index.ts` and its assertion in
  `router/__tests__/settings-routes.spec.ts`.
- The "Radio Stations" nav entry in `layouts/SettingsLayout.vue`.

`StationSearchDialog.vue` and its spec are **moved** (not deleted) to
`components/library/`.

## Tests

New:
- `RadioStationDetailView.spec.ts` — create mode seeds fields from query and calls
  create; edit mode resolves the station, saves, and deletes (delete confirmed);
  not-found state when id is absent after load.
- `RadioStationForm.spec.ts` — validity gating on name/streamUrl, cover selection,
  size-limit error, cover clear.

Updated:
- `RadioView.spec.ts` — Discover and Add buttons present; Add navigates to
  `radio-station-new`; Discover selection navigates with query params.
- `RadioStationCard.spec.ts` / `RadioStationRow.spec.ts` — navigation target; play
  button still plays without navigating.
- Router spec — the two `/radio/...` detail routes resolve; `settings-radio`
  removed.
- Moved `StationSearchDialog.spec.ts`.

## Out of scope / non-goals

- No changes to the Subsonic API or the radio query/mutation hooks.
- No dirty-diffing of cover bytes beyond "a new file was chosen" / "clear set".
- No single-station GET endpoint; the detail view reads from the cached list.
