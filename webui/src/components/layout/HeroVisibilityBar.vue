<script setup lang="ts">
import Button from 'primevue/button'

// A frosted pill for the playlist edit hero, sitting where the selection bubble
// goes when nothing is selected: it toggles the playlist's `public` (shared-read)
// flag. Presentational — the parent owns the value via v-model and stages it
// behind Save alongside the name/description edits.
const model = defineModel<boolean>({ default: false })

const toggle = (): void => {
    model.value = !model.value
}
</script>

<template>
    <div class="edit-visibility" role="group" aria-label="Playlist visibility">
        <Button
            class="ev-toggle"
            :icon="model ? 'pi pi-globe' : 'pi pi-lock'"
            :label="model ? 'Public' : 'Private'"
            severity="secondary"
            text
            :aria-pressed="model"
            @click="toggle"
        />
    </div>
</template>

<style scoped>
/* The frosted pill, mirroring HeroEditSelectionBar's chrome so the visibility
   toggle reads as the same control family on the dark band. It occupies the
   selection bubble's slot whenever no track is selected. */
.edit-visibility {
    align-self: flex-start;
    max-width: 100%;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding: 0.3rem 0.55rem;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 12px;
    backdrop-filter: blur(8px);
}

/* Compact the button to the header controls' size, matching HeroActions/
   HeroEditSelectionBar in the band. */
.edit-visibility :deep(.p-button) {
    min-height: 2.125rem;
    min-width: 0;
    padding-block: 0.15rem;
    padding-inline: 0.55rem;
    font-size: 0.875rem;
}
.edit-visibility :deep(.p-button-icon) {
    font-size: 0.95rem;
}

/* Ghost (secondary text) button stays light on the dark band. */
.edit-visibility :deep(.p-button.p-button-secondary.p-button-text) {
    color: #e2edf4;
}
.edit-visibility :deep(.p-button.p-button-secondary.p-button-text:hover) {
    background: rgba(255, 255, 255, 0.14);
    color: #ffffff;
}
</style>
