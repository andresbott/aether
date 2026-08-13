<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import Toast from 'primevue/toast'
import DesktopShell from '@/layouts/DesktopShell.vue'
import { useQueueSync } from '@/composables/useQueueSync'

const queueSync = useQueueSync()

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
    <DesktopShell />
    <Toast />
</template>
