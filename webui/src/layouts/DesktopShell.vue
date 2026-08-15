<script setup lang="ts">
import PlayerControls from '@/components/layout/PlayerControls.vue'
import ShortcutHelpOverlay from '@/components/layout/ShortcutHelpOverlay.vue'
import { useKeyboardShortcuts } from '@/composables/useKeyboardShortcuts'

// Desktop-only player chrome. PlayerLayout owns the shared skeleton (sidebar,
// route outlet, queue sidebar) so a shell swap never unmounts the active view;
// this component exists so the shortcut listeners stay MOUNT-SCOPED to the
// desktop chrome — the mobile shell (like the settings shell) deliberately
// gets none of them, and the help overlay that teaches them never renders
// there either.
useKeyboardShortcuts()
</script>

<template>
    <PlayerControls />
    <!-- Last, so its badges measure a player bar that is already laid out. -->
    <ShortcutHelpOverlay />
</template>
