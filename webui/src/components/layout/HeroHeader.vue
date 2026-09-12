<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import FileUpload from 'primevue/fileupload'
import Button from 'primevue/button'
import Message from 'primevue/message'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = withDefaults(
    defineProps<{
        eyebrow: string
        editing: boolean
        coverUrl?: string | null
        coverPlaceholderIcon?: string
        coverEditable?: boolean
        coverBackLabel?: string
        coverSizeError?: string | null
        // Clear when there is nothing for Remove to act on — e.g. the served
        // image is a file in the user's music folder, which aether must not
        // touch. The button is hidden rather than disabled.
        coverRemovable?: boolean
        // ArtistView shows a round cover (the artist adaptation). Only in read
        // mode: while editing, the cover flips to a rectangular edit panel so its
        // upload/remove controls are not clipped to a circle.
        roundCover?: boolean
        // AlbumView makes the cover a drag source (drag the album into the queue).
        // Read mode only — the flip-to-edit panel must not be draggable. The host
        // owns what the drag carries via @cover-dragstart / @cover-dragend.
        coverDraggable?: boolean
        coverDragTitle?: string
    }>(),
    {
        coverUrl: null,
        coverPlaceholderIcon: 'pi pi-image',
        coverEditable: true,
        coverBackLabel: 'Cover art',
        coverSizeError: null,
        coverRemovable: true,
        roundCover: false,
        coverDraggable: false,
        coverDragTitle: undefined
    }
)

const emit = defineEmits<{
    (e: 'update:editing', value: boolean): void
    (e: 'cover-select', file: File): void
    (e: 'cover-remove'): void
    (e: 'cover-dragstart', event: DragEvent): void
    (e: 'cover-dragend'): void
}>()

// The 3D flip context (perspective + preserve-3d + a back-rotated face) only
// exists while the cover is `active`. When not editing, the cover is a plain 2D
// image with no 3D layer — this is what keeps it from doing a one-time composite
// "flip" on mount/navigation. `animating` keeps the context (and the transition)
// alive for the duration of the flip back to the front face.
const flipped = ref(props.coverEditable && props.editing)
const animating = ref(false)
const active = computed(() => flipped.value || animating.value)

let timer: ReturnType<typeof setTimeout> | undefined
watch(
    () => props.coverEditable && props.editing,
    (target) => {
        if (target === flipped.value) return
        flipped.value = target
        animating.value = true
        if (timer) clearTimeout(timer)
        timer = setTimeout(() => (animating.value = false), 600)
    }
)
onBeforeUnmount(() => {
    if (timer) clearTimeout(timer)
})

const onSelect = (event: { files: File[] }): void => {
    const file = event.files?.[0]
    if (file) emit('cover-select', file)
}
</script>

<template>
    <!-- Full-bleed hero band: the backdrop is the cover art itself, heavily blurred
         and dimmed into a soft wash of its own colours under a dark scrim (no theme
         tint). The sharp cover and light-ink identity sit on top, so the hero reads
         as a distinct dark panel above the plain track-list body — in BOTH app
         themes (the band is always dark, so its ink is always light). -->
    <div class="hero-header" :class="{ editing }">
        <div class="hero-media" aria-hidden="true">
            <!-- <img> rather than a CSS background-image so the cover url (which
                 carries an apiKey + cache-busting version query string) rides in an
                 attribute and needs no CSS escaping — same reasoning as the
                 flip-back backdrop below. The same url is shown sharp in the cover,
                 so the browser serves both from one request. -->
            <img v-if="coverUrl" class="hero-bg" :src="coverUrl" alt="" />
            <div v-else class="hero-bg hero-bg--fallback"></div>
            <div class="hero-scrim"></div>
        </div>

        <div class="hero-pad">
            <div class="hero-inner content-col">
                <!-- Edit lives in the hero now (it was in the top bar): the host
                     passes its EditActionBar here — a pencil in read mode, then
                     Delete/Save/Cancel while editing, frosted onto the band. -->
                <div v-if="$slots['edit-actions']" class="hero-edit">
                    <slot name="edit-actions" />
                </div>

                <div
                    class="hero-cover"
                    :class="{
                        active,
                        animating,
                        flipped,
                        round: roundCover,
                        grabbable: coverDraggable && !editing
                    }"
                >
                    <div class="flip-inner">
                        <div
                            class="flip-face flip-front"
                            :draggable="coverDraggable && !editing"
                            :title="coverDraggable && !editing ? coverDragTitle : undefined"
                            @dragstart="emit('cover-dragstart', $event)"
                            @dragend="emit('cover-dragend')"
                        >
                            <!-- draggable=false when the cover itself is the drag
                                 source, so grabbing the image fires the host's album
                                 drag rather than the browser's native image drag. -->
                            <img
                                v-if="coverUrl"
                                :src="coverUrl"
                                :alt="eyebrow"
                                :draggable="coverDraggable ? false : undefined"
                            />
                            <div v-else class="cover-placeholder">
                                <i :class="coverPlaceholderIcon" style="font-size: 3rem"></i>
                            </div>
                        </div>
                        <div v-if="coverEditable" class="flip-face flip-back">
                            <!-- The image the controls act on, shown behind a scrim so the
                                 edit panel still reads as a panel. It is the STAGED cover
                                 (`coverUrl`), so a pending upload previews here and a
                                 pending Remove leaves the plain panel — matching what Save
                                 would produce. An <img> rather than a background-image:
                                 the url goes through an attribute instead of into CSS
                                 syntax, so a query string (apiKey, cache-busting version)
                                 needs no escaping. It is its own layer because the scrim
                                 has to sit between the image and the controls. -->
                            <div v-if="coverUrl" class="flip-back-image" aria-hidden="true">
                                <img :src="coverUrl" alt="" />
                            </div>
                            <span class="field-label">{{ coverBackLabel }}</span>
                            <div class="cover-controls">
                                <FileUpload
                                    mode="basic"
                                    accept="image/png,image/jpeg"
                                    :maxFileSize="MAX_COVER_BYTES"
                                    :auto="false"
                                    chooseLabel="Upload image"
                                    @select="onSelect"
                                >
                                    <!-- Suppress PrimeVue's built-in label: it says "No file
                                         chosen" until a file is picked, which contradicts an
                                         image already held on the server. The #cover-note slot
                                         carries the real state, on its own row. -->
                                    <template #filelabel><span /></template>
                                </FileUpload>
                                <!-- Outlined danger rather than a flat secondary text
                                     button: next to the solid upload button, muted text
                                     reads as disabled — and this is a real destructive
                                     action. -->
                                <Button
                                    v-if="coverRemovable"
                                    class="cover-remove"
                                    outlined
                                    severity="danger"
                                    icon="pi pi-trash"
                                    label="Remove"
                                    @click="emit('cover-remove')"
                                />
                                <!-- Optional extra actions, e.g. ArtistView's online image
                                     search. Part of the same stack so it aligns with the
                                     buttons above. -->
                                <slot name="cover-actions" />
                                <Message v-if="coverSizeError" severity="error" :closable="false">
                                    {{ coverSizeError }}
                                </Message>
                                <slot name="cover-note" />
                            </div>
                        </div>
                    </div>
                </div>

                <div class="hero-info">
                    <!-- Empty eyebrow renders nothing (no reserved gap): the
                         playlist editor blanks it while editing so the name form
                         rises to the top of the column. -->
                    <span v-if="eyebrow" class="eyebrow">{{ eyebrow }}</span>
                    <div class="read-only"><slot name="read" /></div>
                    <div class="edit-only"><slot name="edit" /></div>
                    <div v-if="!editing" class="hero-actions-slot"><slot name="actions" /></div>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
/* --- The full-bleed duotone band --------------------------------------------
   The root spans its container edge to edge; `.hero-media` fills it while the
   `.hero-pad` frame reserves the alphabet-rail clearance on the right and the
   `.hero-inner.content-col` centers the content on the shared column, so the
   cover/name/meta line up with the track-list body below. Hosts that mount the
   hero as a FIXED frame above a separate scroller (GenreDetailView) override
   `--hero-rail-clearance` to add the second scrollbar-width (Recipe A). */
.hero-header {
    position: relative;
    isolation: isolate;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.hero-media {
    position: absolute;
    inset: 0;
    overflow: hidden;
    z-index: -1;
    /* A dark base so the scrim math holds even before the cover image paints. */
    background: #0b1017;
}
.hero-bg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    /* Heavily blurred + dimmed so a real, low-resolution cover reads as a soft wash
       of its OWN colours (no theme tint) rather than a pixelated image. The scale
       exceeds the blur radius so the soft edge never uncovers the media base. */
    filter: brightness(0.7) saturate(1.1) blur(30px);
    transform: scale(1.3);
}
/* No cover: a neutral dark wash, theme-independent like the scrimmed covers. */
.hero-bg--fallback {
    background: linear-gradient(135deg, #263243, #0f151d);
    filter: none;
    transform: none;
}
.hero-scrim {
    position: absolute;
    inset: 0;
    background: linear-gradient(
        90deg,
        rgba(8, 11, 17, 0.72) 0%,
        rgba(8, 11, 17, 0.48) 58%,
        rgba(8, 11, 17, 0.34) 100%
    );
}

.hero-pad {
    position: relative;
    box-sizing: border-box;
    padding-right: var(
        --hero-rail-clearance,
        calc(var(--app-rail-clearance) + var(--sb-w, 0px))
    );
}

/* .content-col supplies width / max-width / centering / inline gutter. */
.hero-inner {
    position: relative;
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2rem;
    padding-block: 2.4rem;
    align-items: center;
}

.hero-edit {
    position: absolute;
    top: 1.35rem;
    right: var(--app-content-gutter);
    z-index: 2;
    display: flex;
    align-items: center;
    gap: 0.15rem;
}

/* --- Cover with a 3D flip (front = image, back = edit controls). ---
   The 3D context only exists while `.active` (editing/animating). Outside of it
   the cover is a flat, single-layer image, so it never does a one-time composite
   "flip" on mount or when navigating between items. */
.hero-cover {
    width: 250px;
    height: 250px;
    flex-shrink: 0;
    border-radius: var(--app-radius);
    background: var(--app-bg-subtle);
    overflow: hidden;
    position: relative;
    /* Lift the sharp cover off the inked backdrop. */
    box-shadow: 0 14px 44px rgba(0, 0, 0, 0.55);
}
.hero-cover.active {
    overflow: visible;
    perspective: 1000px;
}
/* Round artist cover, read mode only: the flip context (`.active`) reverts it to
   the rectangular edit panel so the upload/remove controls are not clipped. */
.hero-cover.round:not(.active),
.hero-cover.round:not(.active) .flip-face {
    border-radius: 50%;
}
/* Draggable cover (AlbumView drags the album into the queue): grab affordance. */
.hero-cover.grabbable {
    cursor: grab;
}
.hero-cover.grabbable:active {
    cursor: grabbing;
}
.flip-inner {
    position: relative;
    width: 100%;
    height: 100%;
}
.hero-cover.active .flip-inner {
    transform-style: preserve-3d;
}
.hero-cover.animating .flip-inner {
    transition: transform 0.5s;
}
.hero-cover.flipped .flip-inner {
    transform: rotateY(180deg);
}
.flip-face {
    position: absolute;
    inset: 0;
    border-radius: var(--app-radius);
    overflow: hidden;
}
.hero-cover.active .flip-face {
    backface-visibility: hidden;
}
/* Keep the back face out of the way (and unclickable) until the flip is active. */
.hero-cover:not(.active) .flip-back {
    display: none;
}
.flip-front img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
}
.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}
.flip-back {
    transform: rotateY(180deg);
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    display: flex;
    flex-direction: column;
    align-items: stretch;
    justify-content: center;
    gap: 0.5rem;
    padding: 1rem;
}
/* The staged cover behind the controls, veiled by the scrim. The layer is
   absolutely positioned, so it would otherwise paint OVER the in-flow label and
   buttons (positioned descendants paint after non-positioned ones); the explicit
   z-index pair below is what keeps the controls on top. */
.flip-back-image {
    position: absolute;
    inset: 0;
    z-index: 0;
    overflow: hidden;
    border-radius: var(--app-radius);
    /* Decorative: the same image is already shown, labelled, on the front face. */
    pointer-events: none;
}
.flip-back-image img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
}
.flip-back-image::after {
    content: '';
    position: absolute;
    inset: 0;
    background: var(--app-cover-scrim);
}
.flip-back > :not(.flip-back-image) {
    position: relative;
    z-index: 1;
}
.flip-back .field-label {
    text-align: center;
    margin-bottom: 0.25rem;
}
/* One control per row: the upload button, then Remove, then the status note from
   #cover-note. Keeps a long filename off the button's row inside the 250px face. */
.cover-controls {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 0.35rem;
}
/* Every row spans the same width — the panel's content box — so the upload
   button, Remove and the status note line up as one stack instead of three
   content-sized elements centred at different widths. FileUpload wraps its
   button in a div, so both the wrapper and the button itself need stretching. */
.flip-back :deep(.p-fileupload-basic) {
    display: flex;
}
.flip-back :deep(.p-fileupload-basic-content) {
    flex: 1;
    justify-content: center;
}
.flip-back :deep(.p-fileupload-choose-button),
.cover-controls .cover-remove {
    flex: 1;
    width: 100%;
    justify-content: center;
}

.field-label {
    display: block;
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--app-text-secondary);
}

/* --- Identity column --- */
.hero-info {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    justify-content: center;
    /* Keep long names/meta clear of the absolutely-positioned Edit control. */
    padding-right: 2.75rem;
}
.eyebrow {
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    /* Always light on the inked band, regardless of app theme. */
    color: #8fecff;
}

/* Read/edit swap driven by `.editing` on the root. */
.edit-only {
    display: none;
}
.hero-header.editing .read-only {
    display: none;
}
.hero-header.editing .edit-only {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
}
/* In edit mode the Save/Cancel/Delete trio is absolutely placed top-right, so it
   cannot push the identity column. Top-align the row and clear the trio's height
   so a tall edit form (playlist description, radio URLs) never slides under it. */
.hero-header.editing .hero-inner {
    align-items: start;
}
.hero-header.editing .hero-info {
    padding-top: 3rem;
}

/* Shared typography for slotted identity content (pierces into the slots). All
   light-inked: the band is a dark panel in both themes. */
:deep(.hero-name) {
    margin: 0;
    font-size: 2.4rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    line-height: 1.04;
    color: #f4f8fb;
}
:deep(.hero-desc) {
    margin: 0;
    color: #e2edf4;
    max-width: 46ch;
}
:deep(.meta-row) {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    color: #cdd7df;
    font-size: 0.85rem;
}
:deep(.meta-row .dot)::before {
    content: '•';
    margin-right: 1rem;
    opacity: 0.5;
}
/* Links inside the identity (artist link, playlist owner, etc.) take the light
   cyan rather than the app accent, which is too dark to read on the band. */
.read-only :deep(a) {
    color: #8fecff;
}

/* The action row is a frosted pill in BOTH states — the item's own
   Play/Queue/Favorite and the retargeted selection actions live in the SAME pill,
   so entering or leaving selection only swaps the contents, never the container
   (no height change, no identity jump). Content-width via align-self so it never
   spans the whole column. */
.hero-actions-slot {
    align-self: flex-start;
    max-width: 100%;
    margin-top: 0.7rem;
    padding: 0.4rem 0.6rem;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 12px;
    backdrop-filter: blur(8px);
}
/* The inner rows only lay the buttons out; the pill owns chrome + top spacing. */
.hero-actions-slot :deep(.hero-actions),
.hero-actions-slot :deep(.hero-select) {
    margin-top: 0;
}

/* Compact the action buttons to the header controls' size — a tighter, less
   shouty row on the band. Covers both the item actions (HeroActions) and the
   selection actions (HeroSelectionBar), which share this slot. */
.hero-actions-slot :deep(.p-button) {
    height: 2.125rem;
    padding-block: 0;
    padding-inline: 0.7rem;
    font-size: 0.875rem;
}
.hero-actions-slot :deep(.p-button-icon-only) {
    width: 2.125rem;
    padding: 0;
}
.hero-actions-slot :deep(.p-button-icon) {
    font-size: 0.95rem;
}

/* The play/queue/favorite row (HeroActions) sits on the band: keep its ghost
   (secondary text) buttons light. The primary Play button keeps the accent. */
.hero-actions-slot :deep(.p-button.p-button-secondary.p-button-text) {
    color: #e2edf4;
}
.hero-actions-slot :deep(.p-button.p-button-secondary.p-button-text:hover) {
    background: rgba(255, 255, 255, 0.14);
    color: #ffffff;
}

/* The Edit affordance, frosted onto the band. Read mode is a single pencil given
   a faint frosted chip so it reads as a control on the busy backdrop; edit mode
   is Delete/Save/Cancel with light ink (Delete keeps a lifted danger red). */
.hero-edit :deep(.p-button.p-button-text) {
    color: #e2edf4;
}
.hero-edit :deep(.p-button.p-button-text:hover) {
    background: rgba(255, 255, 255, 0.16);
    color: #ffffff;
}
.hero-edit :deep(.edit-action-edit) {
    background: rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(6px);
}
.hero-edit :deep(.edit-action-delete) {
    color: #ff9aa6;
}
.hero-edit :deep(.edit-action-delete:hover) {
    background: rgba(255, 120, 130, 0.18);
    color: #ffffff;
}

/* Phone: the side-by-side hero becomes a centered stack (spec §3.2), the way
   every mobile music app presents a detail page. 767.98px = $bp-phone-max
   - 0.02px (breakpoints.spec.ts guards the token). */
@media (max-width: 767.98px) {
    .hero-inner {
        grid-template-columns: 1fr;
        justify-items: center;
        text-align: center;
        gap: 1rem;
        padding-block: 1.6rem;
    }

    .hero-cover {
        width: min(60vw, 250px);
        height: auto;
        aspect-ratio: 1;
    }

    .hero-info {
        align-items: center;
        padding-right: 0;
    }

    /* Centered stack on phones: the pill centers with the rest of the identity. */
    .hero-actions-slot {
        align-self: center;
    }

    .hero-edit {
        top: 0.75rem;
    }

    :deep(.hero-name) {
        font-size: 1.6rem;
    }

    :deep(.meta-row) {
        justify-content: center;
    }
}
</style>
