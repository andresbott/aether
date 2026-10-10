<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import ConfirmDialog from 'primevue/confirmdialog'
import Toast from 'primevue/toast'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import QueueSidebar from '@/components/layout/QueueSidebar.vue'
import DesktopShell from '@/layouts/DesktopShell.vue'
import MobileShell from '@/layouts/MobileShell.vue'
import { CHANGE_SERVER_GROUP } from '@/composables/useChangeServer'
import { useQueueSync } from '@/composables/useQueueSync'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'
import { useViewport } from '@/composables/useViewport'
import { useMediaSession } from '@/composables/useMediaSession'
import { useNowPlayingSheet } from '@/composables/useNowPlayingSheet'
import { usePlayer } from '@/composables/usePlayer'
import { isDesktopApp, onTrayCommand } from '@/lib/host'

const route = useRoute()
const queueSync = useQueueSync()
// One chrome at a time: v-if (not CSS hiding) so there is never a duplicate
// tab order or ARIA tree, and shortcuts only bind in the desktop chrome. The
// route outlet lives HERE, outside the swap: a rotation replaces the chrome
// but must never unmount the active view — teardown bypasses the views'
// unsaved-edit guards (onBeforeRouteLeave / beforeunload), which are written
// for navigation, and would silently discard staged edits.
const { shell } = useViewport()
// Recipes read --sb-w in both shells; on phone overlay scrollbars it is 0.
const scrollbarWidth = useScrollbarWidth()

// While the sheet is above collapsed it covers the whole shell: inert takes
// the covered content out of the tab order and the AT tree — the lightweight
// replacement for the focus trap the old overlay needed.
const sheet = useNowPlayingSheet()

// Lock-screen / hardware-key controls; shell-independent, so it lives here
// rather than in either shell.
useMediaSession()

// The desktop app's tray menu lives in Rust, but the player that plays is this
// SPA's: the menu's Play / Pause and Next arrive as events and act through the
// same player actions the media keys use (useMediaSession), so the tray and the
// keys cannot diverge.
const player = usePlayer()
let unlistenTray: (() => void) | undefined
let unmounted = false

onMounted(async () => {
    // Adopt the queue saved from another browser/device before arming the save
    // side: starting first would let a debounced local save race the state
    // arriving from the server.
    await queueSync.restore()
    queueSync.start()
})

onMounted(async () => {
    if (!isDesktopApp()) return
    try {
        const unlisten = await onTrayCommand((command) => {
            if (command === 'play_pause') player.togglePlayPause()
            else if (command === 'next') player.playNext()
        })
        // The layout can unmount while the subscription is still being made: it
        // would never be released, so release it as soon as it exists.
        if (unmounted) unlisten()
        else unlistenTray = unlisten
    } catch (err) {
        // Only the tray menu is lost; the window works without it.
        console.warn('Tray commands unavailable', err)
    }
})

onUnmounted(() => {
    queueSync.stop()
    unmounted = true
    unlistenTray?.()
})
</script>

<template>
    <div class="player-shell" :class="shell === 'desktop' ? 'desktop-shell' : 'mobile-shell'">
        <div class="body-row" :inert="(shell === 'mobile' && sheet.open.value) || undefined">
            <AppSidebar v-if="shell === 'desktop'" />

            <div class="content-area" :style="{ '--sb-w': scrollbarWidth + 'px' }">
                <main class="main-content" :class="{ 'main-content--flush': route.meta.flush }">
                    <RouterView />
                </main>
                <QueueSidebar v-if="shell === 'desktop' && route.name !== 'home'" />
            </div>
        </div>

        <DesktopShell v-if="shell === 'desktop'" />
        <MobileShell v-else />
    </div>
    <Toast />
    <!-- The desktop app's Change server asks through this one dialog, whichever
         shell's account menu raised it (UserMenu on the desktop, the phone's ⋮).
         Here rather than in either menu: it sits outside the shell swap, and its
         own group keeps the ungrouped dialogs the views mount from showing the
         question too. -->
    <ConfirmDialog v-if="isDesktopApp()" :group="CHANGE_SERVER_GROUP" />
</template>

<style>
.player-shell {
    position: relative;
    display: flex;
    flex-direction: column;
    width: 100%;
    overflow: hidden;
    background-color: var(--app-background);
}

/* Both shells take their height from the app-shell chain in _main.scss (html is
   100dvh, body and #app are 100%) rather than measuring the viewport again
   here: a `100vh` column is the URL-bar-hidden height on mobile browsers and
   would hang below the visible area. */
.player-shell.desktop-shell {
    height: 100%;
    padding-bottom: var(--app-player-height);
}

.player-shell.mobile-shell {
    height: 100%;
}

.body-row {
    display: flex;
    flex: 1;
    min-height: 0;
    overflow: hidden;
}

.content-area {
    display: flex;
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
}
</style>
