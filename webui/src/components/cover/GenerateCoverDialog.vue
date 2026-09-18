<script setup lang="ts">
import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ visible: boolean; entityId: string; title?: string }>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'select', pick: { style: string; variation: number }): void
}>()

const candidates = ref<Array<{ style: string; variation: number }>>([])
const loading = ref(false)

async function load() {
    loading.value = true
    try {
        candidates.value = await subsonicClient.getGeneratedCoverCandidates(props.entityId, 9)
    } finally {
        loading.value = false
    }
}
watch(
    () => props.visible,
    (v) => {
        if (v) load()
    },
    { immediate: true }
)

const previewUrl = (c: { style: string; variation: number }) =>
    subsonicClient.getGeneratedCoverPreviewUrl({ id: props.entityId, style: c.style, variation: c.variation, size: 256 })

function choose(c: { style: string; variation: number }) {
    emit('select', c)
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        modal
        header="Generate a cover"
        :style="{ width: '38rem' }"
        @update:visible="emit('update:visible', $event)"
    >
        <div class="candidate-grid">
            <button
                v-for="c in candidates"
                :key="`${c.style}-${c.variation}`"
                type="button"
                class="candidate"
                @click="choose(c)"
            >
                <img :src="previewUrl(c)" :alt="`${c.style} ${c.variation}`" />
            </button>
        </div>
        <template #footer>
            <Button label="Shuffle" icon="pi pi-refresh" text :loading="loading" aria-label="Shuffle" @click="load" />
            <Button label="Cancel" text aria-label="Cancel" @click="emit('update:visible', false)" />
        </template>
    </Dialog>
</template>

<style scoped>
.candidate-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.6rem; }
.candidate { border: 2px solid transparent; border-radius: 8px; padding: 0; cursor: pointer; background: none; }
.candidate:hover { border-color: var(--app-accent); }
.candidate img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 6px; display: block; }
</style>
