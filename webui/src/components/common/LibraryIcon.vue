<script setup lang="ts">
import { computed } from 'vue'
import { LIBRARY_ICON_DIR, resolveLibraryIcon } from '@/lib/libraryIcons'

/**
 * A library's icon — a Material Symbols name chosen at runtime, so it cannot be
 * one of the build-scanned ms-* classes. Masks with the per-icon SVG the build
 * emits under icons/ms/ (served from the embedded SPA, never a CDN); .app-icon
 * shares the ms-* sizing rule, so it lines up with every other icon.
 */
const props = defineProps<{ name?: string }>()

const resolved = computed(() => resolveLibraryIcon(props.name))
const style = computed(() => ({
    '--ms-svg': `url("${import.meta.env.BASE_URL}${LIBRARY_ICON_DIR}/${resolved.value}.svg")`
}))
</script>

<template>
    <i class="app-icon" :data-icon="resolved" :style="style" aria-hidden="true"></i>
</template>
