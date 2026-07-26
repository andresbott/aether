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
    }>(),
    {
        coverUrl: null,
        coverPlaceholderIcon: 'pi pi-image',
        coverEditable: true,
        coverBackLabel: 'Cover art',
        coverSizeError: null,
        coverRemovable: true
    }
)

const emit = defineEmits<{
    (e: 'update:editing', value: boolean): void
    (e: 'cover-select', file: File): void
    (e: 'cover-remove'): void
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
    <div class="hero-header" :class="{ editing }">
        <div class="hero-cover" :class="{ active, animating, flipped }">
            <div class="flip-inner">
                <div class="flip-face flip-front">
                    <img v-if="coverUrl" :src="coverUrl" :alt="eyebrow" />
                    <div v-else class="cover-placeholder">
                        <i :class="coverPlaceholderIcon" style="font-size: 3rem"></i>
                    </div>
                </div>
                <div v-if="coverEditable" class="flip-face flip-back">
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
                    </div>
                    <Message v-if="coverSizeError" severity="error" :closable="false">
                        {{ coverSizeError }}
                    </Message>
                    <slot name="cover-note" />
                </div>
            </div>
        </div>

        <div class="hero-info">
            <span class="eyebrow">{{ eyebrow }}</span>
            <div class="read-only"><slot name="read" /></div>
            <div class="edit-only"><slot name="edit" /></div>
            <div v-if="!editing" class="hero-actions-slot"><slot name="actions" /></div>
        </div>
    </div>
</template>

<style scoped>
.hero-header {
    display: flex;
    gap: 2rem;
    margin-bottom: 2rem;
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
}
.hero-cover.active {
    overflow: visible;
    perspective: 1000px;
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
/* The FileUpload's inner row is left-aligned by default — center it so it lines
   up with the Remove button. */
.flip-back :deep(.p-fileupload-basic-content) {
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
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    justify-content: center;
}
.eyebrow {
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--app-accent);
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

/* Shared typography for slotted identity content (pierces into the slots). */
:deep(.hero-name) {
    margin: 0;
    font-size: 2.4rem;
    font-weight: 800;
    letter-spacing: -0.02em;
}
:deep(.hero-desc) {
    margin: 0;
    color: var(--app-text-secondary);
    max-width: 46ch;
}
:deep(.meta-row) {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
:deep(.meta-row .dot)::before {
    content: '•';
    margin-right: 1rem;
    opacity: 0.5;
}
</style>
