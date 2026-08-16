<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import MobilePlayView from '@/components/layout/MobilePlayView.vue'
import QueueView from '@/components/layout/QueueView.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useViewport } from '@/composables/useViewport'

// `/` is Now Playing in both shells, but the phone mimics the desktop FLOW
// rather than its layout: desktop always shows the queue list (empty state
// included), while the phone renders the play view (MobilePlayView) and,
// with nothing to play, lands on the browse page instead — an empty play view
// is a dead end on a screen that fits only one surface, and /browse is the
// phone's nav surface, so it is where a user with nothing queued starts.
// replace(), not push(): the redirect target stands in for `/`, so back must
// not bounce through it.
const { shell } = useViewport()
const player = usePlayer()
const router = useRouter()

const hasQueue = computed(() => player.queue.value.length > 0)

watch(
    [shell, hasQueue],
    ([currentShell, filled]) => {
        if (currentShell === 'mobile' && !filled) {
            void router.replace({ name: 'browse' })
        }
    },
    { immediate: true }
)
</script>

<template>
    <!-- Nothing renders in the mobile empty-queue gap; the watcher above is
         already replacing the route with the browse page. -->
    <MobilePlayView v-if="shell === 'mobile' && hasQueue" />
    <QueueView v-else-if="shell === 'desktop'" variant="full" />
</template>
