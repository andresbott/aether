# Dense Pro — rollout strategy

Incremental plan to apply the Dense Pro restyle to the real Vue app, **one step
at a time, with a review gate after every step**.

- **Source of truth for values:** [`dense-pro.md`](./dense-pro.md) (token values,
  per-component targets) and the [`dense-pro.html`](./dense-pro.html) reference
  mockup. This file is the *order of operations* and the *working agreement*.
- **Scope:** visual/theme only. No feature changes, no data/logic changes.

---

## How we work (the review loop)

For **each step** below:

1. I make **only** that step's changes (the listed files, nothing else).
2. I report back: **what changed** (files + a short description) and **what to
   look at** (which screens, light *and* dark).
3. I pause. You reply with either:
   - **"approve"** → I commit that step (see git rules) and move to the next, or
   - **change requests** → I adjust, re-report, and wait again. We don't advance
     until you're happy.
4. Nothing moves to the next step or gets committed without your approval.

**Verifying a step**
- Visual: run `npm run dev` and open the affected screens; toggle light/dark.
  (I can also drop headless screenshots into the scratch dir if you prefer to
  review inline.)
- Sanity: `npm run test` after any step that edits component files, to catch
  type errors / broken component specs.

**Git rules (per your conventions)**
- Work on a branch off `main` (e.g. `feat/dense-pro-theme`).
- One commit per approved step. `git add` as its own step, then a single
  one-line commit message. No co-author line. Commit only after you approve.

**Guardrails**
- Keep both themes contrasting at every step (no flat single-tone views).
- Main content views must keep following `docs/architecture/main-content-view-layout.md`
  (ContentScaffold header, flush scroll). Restyle within that structure.

---

## Decisions to lock before we start

These are the open questions from `dense-pro.md`. I'll assume the **recommended**
answer unless you say otherwise — please confirm:

1. **Dark sidebar + player in light theme?** (Recommended: **yes** — it's the
   defining contrast of Dense Pro.) Affects Steps 3 & 4.
2. **Cyan accent** `#0e9bb5` / `#2fd3ef` replacing indigo everywhere? (Recommended: **yes**.)
3. **Add a third surface token** `--app-surface-2` for cards? (Recommended: **yes**.)
4. **Density scope:** compact spacing applied broadly, or mainly to lists/queue/
   cards? (Recommended: **lists/queue/cards**, leave hero/reading areas relaxed.)

---

## Sequence overview

| # | Step | Main file(s) | Risk |
|---|------|-------------|------|
| 1 | Color tokens + new tokens | `_variables.scss` | low, wide visual impact |
| 2 | Dimensions | `_variables.scss` | low |
| 3 | Sidebar (dark rail, full-width active, right bar) | `AppSidebar.vue` | medium (defining change) |
| 4 | Player bar (taller, roomier, cyan) | `PlayerControls.vue` | low |
| 5 | Queue (soft divider, full-width current, left bar) | `QueueSidebar.vue`, `QueueRow.vue` | low |
| 6 | Track lists (compact rows, clock header) | `AlbumTrackRow.vue` + track headers | medium |
| 7 | Library card grid (denser cards) | `VirtualCardGrid.vue` + card component | low |
| 8 | ContentScaffold + hero density | `ContentScaffold.vue`, view heroes | low |
| 9 | Polish & QA sweep | across | low |

Order rationale: tokens first give an immediate, whole-app recolor to review; then we
restyle one region at a time from the frame inward (nav → player → queue → lists
→ cards → headers), each visually isolated and easy to judge.

---

## Steps

### Step 1 — Color tokens
**Goal:** recolor the whole app to the Dense Pro palette; define new tokens.
**File:** `src/assets/scss/_variables.scss`.
**Changes:** update every existing color token's light + dark value per the table
in `dense-pro.md §1` (accent indigo→cyan, layered slate/grey surfaces, softer
borders, darker player). Add the new tokens: `--app-surface-2`, `--app-nav-bg`,
`--app-nav-text`, `--app-nav-text-dim`, `--app-nav-brand`, `--app-player-dim`,
`--app-player-track` (defined now, consumed in later steps).
**Review:** every screen recolors; sidebar/player still light-styled (that's
Step 3/4). Check text contrast in both themes.
**Done when:** palette matches the mockup's colors; nothing structurally moved.

### Step 2 — Dimensions
**Goal:** apply the denser measurements.
**File:** `_variables.scss`.
**Changes:** `--app-content-max-width` → `1320px`, `--app-player-height` → `116px`,
add a radius token (~`8px`) if we standardize corners. (Optional sidebar width
tweak toward `240px`.)
**Review:** content column narrows slightly; player row reserves more height.
**Done when:** spacing matches without any component-level edits yet.

### Step 3 — Sidebar  *(the defining change — needs Decision 1)*
**Goal:** dark navigation rail in both themes, full-width selected items.
**File:** `src/components/layout/AppSidebar.vue`.
**Changes:** background `--app-nav-bg` (dark) in both themes; text via
`--app-nav-text`/`-dim`; brand cyan. Selected items become **full-width bars**
(remove the nav list's horizontal padding + item `border-radius: 0`); active =
cyan text on full-width `--app-accent-soft` with a **right-edge bar**
(`inset -3px 0 0 var(--app-accent)`), flush to the sidebar's right border. Apply
to the collapsed rail too. Tighten item padding/font.
**Review:** big shift — dark rail against light content in light mode; confirm the
full-width active bar and right highlight on every nav section + collapsed state.
**Done when:** matches the mockup's sidebar in both themes.

### Step 4 — Player bar
**Goal:** dark, roomier transport bar.
**File:** `src/components/layout/PlayerControls.vue`.
**Changes:** background `--app-player-bg`; height `116px` with **~24px vertical
padding** so play/pause + scrubber breathe (was cramped); larger gap (~11px)
between transport row and scrubber; cyan play button; muted icons use
`--app-player-dim`; rails use `--app-player-track`; times tabular-nums.
**Review:** the bar sits taller with clear space above/below the controls.
**Done when:** matches the mockup's player.

### Step 5 — Queue
**Goal:** match the sidebar treatment on the queue.
**Files:** `src/components/layout/QueueSidebar.vue`, `QueueRow.vue`.
**Changes:** panel divider (`border-left`) uses the **soft `--app-border`** (low
contrast, not a dark edge). Rows go **full-width** (drop list horizontal padding
+ row `border-radius: 0`). Current item = full-width `--app-accent-soft` with a
**left-edge bar** (`inset 3px 0 0 var(--app-accent)`, the content-facing edge).
Compact row height; count badge accent-soft + cyan.
**Review:** current track reads as a full-width bar with a left cyan edge; the
panel divider is subtle.
**Done when:** matches the mockup's queue.

### Step 6 — Track lists
**Goal:** compact, tabular track rows.
**Files:** `src/components/library/AlbumTrackRow.vue` and the album/artist track
headers (e.g. in `AlbumView.vue`).
**Changes:** row height ~`40px`; tighten grid columns
(`38px minmax(0,2.4fr) minmax(0,1.4fr) 34px 62px`); duration header shows a small
clock icon; durations `tabular-nums`; current/playing row = `--app-accent-soft`
with cyan title + cyan index (volume icon in place of number).
**Review:** denser rows; playing row highlighted; columns aligned.
**Done when:** matches the mockup's tracklist.

### Step 7 — Library card grid
**Goal:** denser cards.
**Files:** `src/components/library/VirtualCardGrid.vue` + the card component it
renders.
**Changes:** smaller min card width (~`142–160px`), gap `12px`, radius `8px`,
padding `8px`; card = `--app-surface-2` with a hairline `--app-border` and a
subtle shadow, lifting on hover; cover-art radius ~`5px`.
**Review:** more albums per row; subtle elevation; hover lift.
**Done when:** matches the mockup's library grid.

### Step 8 — ContentScaffold + hero density
**Goal:** tighter headers and heroes.
**Files:** `src/components/layout/ContentScaffold.vue`; view heroes
(`AlbumView.vue`, artist, song).
**Changes:** smaller title (~`22px`), tighter header spacing, summary line in
`--app-text-secondary`. Heroes stay flat on `--app-surface-2` (no gradients —
that experiment was dropped).
**Review:** headers/heroes feel compact and consistent across views.
**Done when:** matches the mockup's scaffold/heroes.

### Step 9 — Polish & QA sweep
**Goal:** catch stragglers.
**Changes:** sweep for any remaining indigo/old-accent usages, hover/focus states,
scrollbars, disabled states; verify dark-mode contrast everywhere; re-run
`npm run test` and `npm run type-check`.
**Review:** full walk-through of all views in both themes.
**Done when:** no leftover old styling; tests green.

---

## Notes
- Each step is intentionally small and self-contained, so any step can be reverted
  in isolation if you change your mind.
- If a step reveals the mockup needs a tweak, we update `dense-pro.html` +
  `dense-pro.md` first, re-confirm, then apply to the app — mockup stays the
  reference.
