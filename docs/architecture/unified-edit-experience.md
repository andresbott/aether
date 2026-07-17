# Unified Edit Experience — Canonical Behavior

Every editable **detail view** (a single-item main content view) shares one edit
affordance. This is the registry of that behavior; the implementation lives in
`webui/src/components/layout/EditActionBar.vue` and one global CSS rule.

## The affordance

- **Read mode:** the view's own header actions (Play, Star, Add to Queue, …) plus a
  **pencil** icon button (`pi pi-pencil`, no text, tooltip "Edit"). Clicking it enters
  edit mode.
- **Edit mode:** three **icon-only** buttons with tooltips, and the read-mode actions are
  hidden:
  - **Save** — `pi pi-check`, tooltip "Save" (or "Create" when creating a new item).
  - **Cancel** — `pi pi-times`, tooltip "Cancel".
  - **Delete** — `pi pi-trash`, `severity="danger"`, tooltip "Delete".
- **Delete** opens a confirmation dialog ordered **confirm | cancel** (confirm on the
  left, styled danger). Confirming performs the delete; cancelling closes the dialog.

## Single sources of truth

- **`EditActionBar`** (`webui/src/components/layout/EditActionBar.vue`) renders the whole
  set into `ContentScaffold`'s `#actions` slot and owns the delete confirmation.
  - Props: `editing` (v-model), `saveDisabled`, `saving`, `canDelete` (default `true`),
    `deleteHeader`, `deleteMessage`, `saveIcon`, `saveTooltip`.
  - Emits: `update:editing`, `save`, `cancel`, `delete` (emitted only after the user
    confirms). The **view** owns state and decides when to leave edit mode (a save may be
    async or fail), so the bar never flips `editing` itself except via the pencil.
  - Slot `#read-actions` holds view-specific read-mode buttons; hidden in edit mode.
  - Stable test/hook classes: `.edit-action-edit`, `.edit-action-save`,
    `.edit-action-cancel`, `.edit-action-delete`.
- **Confirm-first ordering** is a single global rule in
  `webui/src/assets/scss/_main.scss` (`.p-confirmdialog .p-dialog-footer { display: flex;
  flex-direction: row-reverse }`). It is visual-only — keyboard focus still reaches Cancel first, the safer
  default for a destructive action — and applies to every confirm dialog app-wide.

## How views adapt

A view shows only the buttons that apply:

| View | Route | Editable | Delete? | Notes |
|------|-------|----------|---------|-------|
| Radio station | `/radio/:id`, `/radio/new` | name, stream, homepage, cover | yes | Create mode (`/radio/new`) starts in edit mode, `canDelete=false`, Save reads "Create", Cancel returns to `/radio`. |
| Playlist | `/playlist/:id` | name, description, cover, tracks | yes | Track reordering is always-on; identity/cover/track edits persist together on Save. |
| Artist | `/artist/:id` | cover only | no | `canDelete=false`. Cover edits are **staged** and applied on Save. |
| Album | `/album/:id` | — | — | **Not editable** (album data derives from file metadata); no pencil. |

Rules for a new editable detail view:

1. Import `EditActionBar` and place it in `ContentScaffold`'s `#actions`, read-mode extras
   in `#read-actions`.
2. Stage edits locally; gate Save with a `dirty`/`valid` computed via `save-disabled`, and
   pass the in-flight mutation state via `saving`.
3. `@save` persists then leaves edit mode; `@cancel` discards staged edits and leaves edit
   mode; `@delete` runs the delete mutation (the bar already confirmed).
4. Set `canDelete=false` when the entity cannot be deleted.
5. Keep a `<ConfirmDialog />` mounted in the view (the bar's `useConfirm` renders through it).
6. Add unsaved-changes guards (`onBeforeRouteLeave` + `beforeunload`) when edits are staged.
