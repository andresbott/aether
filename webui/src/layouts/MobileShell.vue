<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import MiniPlayer from '@/components/layout/MiniPlayer.vue'
import MobileTabBar from '@/components/layout/MobileTabBar.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useScrollbarWidth } from '@/composables/useScrollbarWidth'

const route = useRoute()
const { queue } = usePlayer()
// Recipes still read --sb-w on mobile; on phone overlay scrollbars it is 0.
const scrollbarWidth = useScrollbarWidth()
</script>

<template>
    <div class="mobile-shell">
        <div class="mobile-content" :style="{ '--sb-w': scrollbarWidth + 'px' }">
            <main class="main-content" :class="{ 'main-content--flush': route.meta.flush }">
                <RouterView />
            </main>
        </div>
        <!-- Docked chrome: mini-player only while there is something to play. -->
        <MiniPlayer v-if="queue.length > 0" />
        <MobileTabBar />
    </div>
</template>

<style>
.mobile-shell {
    display: flex;
    flex-direction: column;
    /* dvh, not vh: mobile browser UI bars shrink the visible viewport, and a
       100vh column would push the tab bar under them. */
    height: 100dvh;
    width: 100%;
    overflow: hidden;
    background-color: var(--app-background);
}

.mobile-content {
    display: flex;
    flex: 1;
    min-height: 0;
    overflow: hidden;
}
</style>
