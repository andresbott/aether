<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PlayerLayout from '@/layouts/PlayerLayout.vue'
import AdminLayout from '@/layouts/AdminLayout.vue'

declare module 'vue-router' {
    interface RouteMeta {
        layout?: 'player' | 'admin'
    }
}

const route = useRoute()

const layouts = {
    player: PlayerLayout,
    admin: AdminLayout,
} as const

const layout = computed(() => layouts[route.meta.layout ?? 'player'])
</script>

<template>
    <component :is="layout" />
</template>
