<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Checkbox from 'primevue/checkbox'
import Button from 'primevue/button'
import { useGeneratedCoverSettings, useUpdateGeneratedCoverSettings } from '@/composables/useGeneratedCoverSettings'
import { subsonicClient } from '@/lib/api/subsonic'

const { data, isLoading } = useGeneratedCoverSettings()
const save = useUpdateGeneratedCoverSettings()

const defaultSet = ref<string[]>([])
const availableSet = ref<string[]>([])
watch(
    data,
    (d) => {
        if (d) {
            defaultSet.value = [...d.default]
            availableSet.value = [...d.available]
        }
    },
    { immediate: true }
)

const styles = computed(() => data.value?.styles ?? [])
// A generated placeholder must always be possible: block saving an empty Default set.
const canSave = computed(() => defaultSet.value.length > 0)
const previewUrl = (style: string) =>
    subsonicClient.getGeneratedCoverPreviewUrl({ style, variation: 0, size: 96 })

function onSave() {
    if (!canSave.value) return
    save.mutate({ default: defaultSet.value, available: availableSet.value })
}
</script>

<template>
    <section class="section">
        <div class="section-header">
            <h2>Generated Covers</h2>
            <Button label="Save" :disabled="!canSave || save.isPending.value" @click="onSave" />
        </div>
        <p v-if="isLoading" class="loading"><i class="pi pi-spin pi-spinner" /></p>
        <div v-else class="cover-style-groups">
            <div class="style-group">
                <h3>Default (auto-generated placeholders)</h3>
                <label v-for="s in styles" :key="`d-${s.name}`" class="style-row">
                    <Checkbox v-model="defaultSet" :value="s.name" />
                    <img class="style-thumb" :src="previewUrl(s.name)" :alt="s.label" />
                    <span>{{ s.label }}</span>
                </label>
                <p v-if="!canSave" class="field-hint">Select at least one default style.</p>
            </div>
            <div class="style-group">
                <h3>Available (offered when editing a cover)</h3>
                <label v-for="s in styles" :key="`a-${s.name}`" class="style-row">
                    <Checkbox v-model="availableSet" :value="s.name" />
                    <img class="style-thumb" :src="previewUrl(s.name)" :alt="s.label" />
                    <span>{{ s.label }}</span>
                </label>
            </div>
        </div>
    </section>
</template>

<style scoped>
.cover-style-groups { display: flex; gap: 2rem; flex-wrap: wrap; }
.style-group { min-width: 16rem; }
.style-row { display: flex; align-items: center; gap: 0.6rem; padding: 0.35rem 0; }
.style-thumb { width: 48px; height: 48px; border-radius: 6px; object-fit: cover; }
.field-hint { color: var(--app-text-secondary); font-size: 0.85rem; }
</style>
