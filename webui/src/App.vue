<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PlayerLayout from '@/layouts/PlayerLayout.vue'
import SettingsLayout from '@/layouts/SettingsLayout.vue'

declare module 'vue-router' {
    interface RouteMeta {
        layout?: 'player' | 'settings'
        // Full-bleed main content view: the view owns its own horizontal gutter
        // and self-scrolls, so PlayerLayout drops the shell's side padding.
        flush?: boolean
    }
}

const route = useRoute()

const layouts = {
    player: PlayerLayout,
    settings: SettingsLayout,
} as const

const layout = computed(() => layouts[route.meta.layout ?? 'player'])
</script>

<template>
    <component :is="layout" />
</template>
