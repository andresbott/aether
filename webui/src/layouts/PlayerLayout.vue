<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import Toast from 'primevue/toast'
import DesktopShell from '@/layouts/DesktopShell.vue'
import MobileShell from '@/layouts/MobileShell.vue'
import { useQueueSync } from '@/composables/useQueueSync'
import { useViewport } from '@/composables/useViewport'
import { useMediaSession } from '@/composables/useMediaSession'

const queueSync = useQueueSync()
// One chrome at a time: v-if (not CSS hiding) so there is never a duplicate
// tab order or ARIA tree, and shortcuts only bind in the desktop chrome.
const { shell } = useViewport()

// Lock-screen / hardware-key controls; shell-independent, so it lives here
// rather than in either shell.
useMediaSession()

onMounted(async () => {
    // Adopt the queue saved from another browser/device before arming the save
    // side: starting first would let a debounced local save race the state
    // arriving from the server.
    await queueSync.restore()
    queueSync.start()
})

onUnmounted(() => {
    queueSync.stop()
})
</script>

<template>
    <DesktopShell v-if="shell === 'desktop'" />
    <MobileShell v-else />
    <Toast />
</template>
