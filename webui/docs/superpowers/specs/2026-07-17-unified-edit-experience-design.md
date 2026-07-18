# Unified Edit Experience — Design

Date: 2026-07-17

## Goal

Give every editable **main content view** the same edit affordance:

- A **pencil icon button (no text)** in the view header. Clicking it switches the view
  into **edit mode**.
- In edit mode the header shows three **icon-only** buttons: **Save** (`pi pi-check`),
  **Cancel** (`pi pi-times`), **Delete** (`pi pi-trash`, danger). Each has a tooltip.
- **Delete** opens a confirmation dialog whose buttons are ordered **confirm | cancel**
  (confirm on the left), with the confirm button styled danger.
- While in edit mode, the view's read-mode header actions (Play, Star, Add to Queue, …)
  are hidden — only Save/Cancel/Delete show.

Views that cannot delete or cannot be created (see per-view notes) show only the subset of
buttons that applies.

## Scope

Editable detail views under `PlayerLayout`:

| View | Route | Editable fields | Delete? | Create? |
|------|-------|-----------------|---------|---------|
| `RadioStationDetailView` | `/radio/:id`, `/radio/new` | name, stream URL, homepage, cover | yes | yes (`/radio/new`) |
| `PlaylistDetailView` | `/playlist/:id` | name, description, cover, tracks | yes | no |
| `ArtistView` | `/artist/:id` | cover only | no | no |

**Out of scope:** `AlbumView` (`/album/:id`) is not editable (album data is derived from
file metadata); it gets no edit affordance. All non-detail main content views (Library,
Search, Radio list, Playlists list, Genres, Home, Now Playing) are unaffected.

## Approach

A shared presentational component **`EditActionBar`** renders the header action set for any
editable detail view, so the button set, icons, tooltips, edit-mode hiding, and delete
confirmation live in exactly one place. Each view composes it inside `ContentScaffold`'s
`#actions` slot.

Rejected alternatives:
- *Inline convention only* — three copies to keep in sync; the drift we are fixing.
- *Bake edit mode into `ContentScaffold`* — couples the deliberately domain-agnostic scaffold
  to edit semantics; the layout doc keeps the scaffold generic.

## Component: `EditActionBar.vue`

Location: `webui/src/components/layout/EditActionBar.vue`.

**Props**
- `editing: boolean` (v-model) — current mode.
- `saveDisabled?: boolean` (default `false`) — disables Save.
- `saving?: boolean` (default `false`) — Save shows loading spinner.
- `canDelete?: boolean` (default `true`) — when `false`, Delete is not rendered.
- `deleteHeader?: string` — confirmation dialog header (e.g. "Delete station?").
- `deleteMessage?: string` — confirmation dialog message.
- `saveIcon?: string` (default `pi pi-check`).
- `saveTooltip?: string` (default `Save`).

**v-model / emits**
- `update:editing` — pencil sets `true`; internal Save/Cancel do **not** flip it (the view
  decides, since a save may be async/fail). Emits are the contract; the view flips `editing`.
- `save` — Save clicked.
- `cancel` — Cancel clicked.
- `delete` — emitted only after the user confirms in the dialog.

**Slots**
- `#read-actions` — view-specific buttons (Play, Star, Add to Queue). Rendered **only** in
  read mode.

**Structure**
- Read mode: `<slot name="read-actions" />` then a `pi pi-pencil` icon-only Button
  (`text rounded`, tooltip "Edit"), `@click` → `emit('update:editing', true)`.
- Edit mode: Save (`text rounded`, `:icon="saveIcon"`, `:disabled="saveDisabled"`,
  `:loading="saving"`, tooltip `saveTooltip`) → `emit('save')`; Cancel (`text rounded`,
  `pi pi-times`, tooltip "Cancel") → `emit('cancel')`; Delete when `canDelete`
  (`text rounded severity="danger"`, `pi pi-trash`, tooltip "Delete") → `confirmDelete()`.
- `confirmDelete()` calls `useConfirm().require({ header: deleteHeader, message:
  deleteMessage, icon: 'pi pi-exclamation-triangle', acceptClass: 'p-button-danger',
  acceptLabel: 'Delete', rejectLabel: 'Cancel', accept: () => emit('delete') })`.

The component does **not** render its own `<ConfirmDialog>`; the host view keeps mounting one
(a single app-wide service instance drives them).

## Global confirm-first dialog ordering

PrimeVue's `ConfirmDialog` renders reject (Cancel) then accept (Delete), i.e. `Cancel |
Delete`. We want `confirm | cancel`. Apply one **global** stylesheet rule reversing the
dialog footer:

```css
.p-confirmdialog .p-dialog-footer { flex-direction: row-reverse; }
```

This flips the **visual** order to `Delete | Cancel` for every confirm dialog app-wide
(Playlist, Radio, and the settings `LibrariesPanel`), keeping the app consistent. It is
visual-only: keyboard tab order still reaches Cancel first, which is the safer default focus
for a destructive action. Lives in the app's global SCSS (alongside other PrimeVue
overrides), not in a scoped component style.

## Per-view changes

### PlaylistDetailView
- Replace the inline read/edit action markup with `EditActionBar`.
  - `#read-actions`: the existing Play button.
  - `v-model:editing`, `:save-disabled` from existing gate, `:saving="savePending"`,
    `delete-header="Delete playlist?"`, `delete-message` = existing string.
  - `@save="saveEdit"`, `@cancel="cancelEdit"`, `@delete="handleDelete"` (drop the internal
    `confirm.require` in `handleDelete`; the bar owns confirmation and emits `delete` →
    the view just runs the mutation).
- Save/Cancel become icon-only (were labeled). Staged name/description/cover/track behavior,
  dirty computeds, and unsaved-change guards are unchanged.

### RadioStationDetailView
- Replace the current pencil/check toggle + always-visible Save + trash with `EditActionBar`.
- **Edit mode** (`/radio/:id`): `#read-actions` = Play; `v-model:editing`;
  `:save-disabled="!valid"`; `:saving="submitting"`; `delete-header="Delete station?"`;
  `delete-message` = existing; `@save="onSave"` (flips `editing=false` on success as today),
  `@cancel` reseeds form from station + resets cover staging + `editing=false`,
  `@delete="onDelete"` (drop internal confirm; run delete mutation on emit).
- **Create mode** (`/radio/new`): no read mode (`editing` starts `true`), `canDelete=false`.
  Save acts as create (`saveIcon="pi pi-check"`, `saveTooltip="Create"`, `@save="onCreate"`),
  Cancel navigates back to `/radio`. A dedicated `onCancel` handles both: reseed in edit
  mode, `router.push({ name: 'radio' })` in create mode.

### ArtistView
- Adopt the pattern. Cover editing changes from **immediate** to **staged**:
  - Add staged-cover state mirroring Playlist/Radio: `selectedFile`, `previewUrl`,
    `coverClear`, `coverSizeError`; `displayedCoverUrl` prefers the preview.
  - `dirty` = a cover file staged or a clear staged.
  - Save applies via `useUpdateArtistCover` (file or `coverClear`), then resets staging,
    bumps `cacheBust`, `editing=false`. Cancel discards staging + `editing=false`.
  - Add the same unsaved-changes guards (`onBeforeRouteLeave`, `beforeunload`) as the others.
- `EditActionBar`: `#read-actions` = the existing Star button; `canDelete=false`;
  `:save-disabled` when not dirty (optional) ; `@save`, `@cancel`.

### AlbumView
- No change. Documented as intentionally non-editable.

## Testing

- **`EditActionBar.spec.ts`** (new): pencil shown in read mode and emits `update:editing`
  true on click; in edit mode renders Save + Cancel (+ Delete); Delete hidden when
  `canDelete=false`; Save disabled/loading reflect props; `save`/`cancel` emitted; Delete
  routes through the confirm service and emits `delete` only on accept; `#read-actions`
  hidden in edit mode.
- **PlaylistDetailView.spec.ts / RadioStationDetailView.spec.ts**: update selectors for the
  now icon-only Save/Cancel; assert edit-mode button set; delete flow still works via the
  bar. Keep create-mode Radio assertions (Create + Cancel, no Delete).
- **ArtistView.spec.ts**: staged cover edit — select file shows preview, Cancel discards,
  Save calls the mutation; no Delete button present; dirty guard fires.

## Registry doc

New `docs/architecture/unified-edit-experience.md` records the canonical behavior (pencil →
edit; icon-only Save/Cancel/Delete with tooltips; confirm-first delete dialog; how views
without delete/create adapt; the `EditActionBar` component and the global footer CSS rule as
the single sources of truth). Cross-link it from
`docs/architecture/main-content-view-layout.md`.
