<script setup lang="ts">
import { ref, computed } from 'vue'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'
import type { Pair } from './useEditForm'
import type { ArtistMatchPayload } from '@/types/artists'

const props = defineProps<{
    modelValue: Pair[]
    mixed: boolean
    dirty: boolean
    labels: {
        heading: string
        namePlaceholder: string
        mbidPlaceholder: string
        addLabel: string
        removeAriaLabel: string
    }
    undoTooltip: string
    undoTestId?: string
    undoAriaLabel?: string
    mixedNote: string
}>()

const emit = defineEmits<{
    (e: 'add'): void
    (e: 'remove', index: number): void
    (e: 'stage'): void
    (e: 'undo'): void
    (e: 'pick', index: number, payload: ArtistMatchPayload): void
}>()

// Local refs for the pair inputs, synced with the prop
const pairs = computed({
    get: () => props.modelValue,
    set: () => emit('stage')
})

// Picker dialog state targeting a specific pair by index
const pickerState = ref<{ open: boolean; index: number }>({ open: false, index: -1 })
const pickerPair = computed<Pair | undefined>(() => props.modelValue[pickerState.value.index])

function openPicker(index: number) {
    pickerState.value = { open: true, index }
}

function onPickerSelect(payload: ArtistMatchPayload) {
    emit('pick', pickerState.value.index, payload)
    pickerState.value.open = false
}

function mbidPlaceholder(pair: Pair): string {
    return pair.mixed && !pair.mbid ? '(mixed)' : ''
}
</script>

<template>
    <div class="pairs">
        <small v-if="mixed" class="mixed-note">
            {{ mixedNote }}
        </small>
        <div v-for="(pair, i) in pairs" :key="i" class="pair">
            <div class="pair-fields">
                <div class="pair-field">
                    <label>{{ labels.heading }}</label>
                    <InputText
                        class="pair-name"
                        v-model="pair.name"
                        :placeholder="labels.namePlaceholder"
                        @update:modelValue="emit('stage')"
                    />
                </div>
                <div class="pair-field">
                    <label>{{ labels.mbidPlaceholder }}</label>
                    <InputText
                        class="pair-mbid"
                        v-model="pair.mbid"
                        :placeholder="mbidPlaceholder(pair)"
                        @update:modelValue="emit('stage')"
                    />
                </div>
            </div>
            <div class="pair-actions">
                <Button
                    icon="pi pi-search"
                    text
                    size="small"
                    aria-label="Search MusicBrainz"
                    @click="openPicker(i)"
                />
                <Button
                    icon="pi pi-times"
                    text
                    size="small"
                    severity="secondary"
                    :aria-label="labels.removeAriaLabel"
                    @click="emit('remove', i)"
                />
            </div>
        </div>
        <Button
            icon="pi pi-plus"
            :label="labels.addLabel"
            text
            size="small"
            @click="emit('add')"
        />
    </div>

    <MusicBrainzArtistPicker
        v-model:visible="pickerState.open"
        :artistName="pickerPair?.name ?? ''"
        :currentMbid="pickerPair?.mbid ?? ''"
        @select="onPickerSelect"
    />
</template>

<style scoped>
.pairs {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}
.pair {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.6rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
.pair-fields {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    min-width: 0;
}
.pair-field {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
}
.pair-field label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    color: var(--app-text-secondary);
}
.pair-actions {
    display: flex;
    align-items: center;
    gap: 0.15rem;
}
.pair :deep(.p-inputtext) {
    width: 100%;
    font-size: 0.85rem;
}
.pair-mbid {
    font-family: var(--font-mono, monospace);
}
.mixed-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
</style>
