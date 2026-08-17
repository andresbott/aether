<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import QueueView from '@/components/layout/QueueView.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useViewport } from '@/composables/useViewport'

// `/` is Now Playing in both shells. Desktop renders the queue list; the
// mobile shell has no page here at all — Now Playing is the sheet
// (NowPlayingSheet), addressed by hash on whatever route sits underneath. So
// on mobile `/` is only an ADDRESS: it replaces itself with the landing page
// carrying the sheet's hash (#playing) when something is queued, or the bare
// landing page when not. replace(), not push(): the target stands in for `/`,
// so back must not bounce through it.
const { shell } = useViewport()
const player = usePlayer()
const router = useRouter()

const hasQueue = computed(() => player.queue.value.length > 0)

watch(
    [shell, hasQueue],
    ([currentShell, filled]) => {
        if (currentShell !== 'mobile') return
        void router.replace(filled ? { name: 'browse', hash: '#playing' } : { name: 'browse' })
    },
    { immediate: true }
)
</script>

<template>
    <QueueView v-if="shell === 'desktop'" variant="full" />
</template>
