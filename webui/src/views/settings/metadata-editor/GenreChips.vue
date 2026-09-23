<script setup lang="ts">
import { computed, useId } from 'vue'
import AutoComplete from 'primevue/autocomplete'
import Button from 'primevue/button'

const props = defineProps<{
    modelValue: string[]
    mixed: boolean
    dirty: boolean
    undoTooltip: string
}>()

const emit = defineEmits<{
    (e: 'update:modelValue', value: string[]): void
    (e: 'undo'): void
}>()

const uid = useId()
const inputId = `${uid}-genres`

const genres = computed({
    get: () => props.modelValue,
    set: (v) => emit('update:modelValue', v)
})
</script>

<template>
    <div
        class="field-block"
        :class="{ 'section-dirty': dirty }"
        data-test="genres-block"
    >
        <label :for="inputId">
            Genres
            <Button
                v-if="dirty"
                icon="ms-undo"
                text
                size="small"
                aria-label="Reset genres"
                data-test="undo-genres"
                v-tooltip.left="undoTooltip"
                @click="emit('undo')"
            />
        </label>
        <div class="genres-field">
            <small v-if="mixed" class="mixed-note" data-test="genres-mixed">
                Selected tracks have different genres. Add genres to overwrite all of
                them; leave empty to keep each track's own.
            </small>
            <AutoComplete
                :inputId="inputId"
                v-model="genres"
                multiple
                :typeahead="false"
                placeholder="Add genre and press Enter"
                data-test="genres-input"
            />
        </div>
    </div>
</template>

<style scoped>
.field-block {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: start;
    gap: 0.5rem;
}
.field-block label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    padding-top: 0.35rem;
}
.field-block.section-dirty > label {
    color: var(--app-staged);
}
.genres-field {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}
.genres-field :deep(.p-autocomplete) {
    width: 100%;
}
.genres-field :deep(.p-autocomplete-input-multiple) {
    width: 100%;
}
.field-block.section-dirty .genres-field :deep(.p-autocomplete-input-multiple) {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.mixed-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
</style>
