<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MobileMoreDrawer from '@/components/layout/MobileMoreDrawer.vue'

interface Tab {
    label: string
    icon: string
    routeName: string
}

// The four primary destinations; everything else lives in the More drawer.
// Icons match the desktop sidebar so the two shells read as one app.
const TABS: Tab[] = [
    { label: 'Home', icon: 'pi pi-play-circle', routeName: 'home' },
    { label: 'Library', icon: 'pi pi-compass', routeName: 'library' },
    { label: 'Search', icon: 'pi pi-search', routeName: 'search' },
    { label: 'Playlists', icon: 'pi pi-list', routeName: 'playlists' }
]

const route = useRoute()
const router = useRouter()
const moreOpen = ref(false)

const isActive = (tab: Tab): boolean => route.name === tab.routeName

// Named routes with no params — same contract as the keyboard shortcuts:
// `library` without a folderId is the cross-collection root.
const go = (tab: Tab): void => {
    void router.push({ name: tab.routeName })
}
</script>

<template>
    <nav class="mobile-tab-bar" aria-label="Primary">
        <button
            v-for="tab in TABS"
            :key="tab.routeName"
            type="button"
            class="tab-item"
            :class="{ active: isActive(tab) }"
            @click="go(tab)"
        >
            <i :class="tab.icon"></i>
            <span class="tab-label">{{ tab.label }}</span>
        </button>
        <button
            type="button"
            class="tab-item"
            aria-haspopup="dialog"
            :aria-expanded="moreOpen"
            @click="moreOpen = true"
        >
            <i class="pi pi-ellipsis-h"></i>
            <span class="tab-label">More</span>
        </button>
        <MobileMoreDrawer v-model:visible="moreOpen" />
    </nav>
</template>

<style scoped>
.mobile-tab-bar {
    display: flex;
    height: var(--app-mobile-tabbar-height);
    flex-shrink: 0;
    background-color: var(--app-nav-bg);
    border-top: 1px solid var(--app-border);
}

.tab-item {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.15rem;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-nav-text-dim);
    font-size: 0.65rem;
    font-weight: 600;
}

.tab-item i {
    font-size: 1.2rem;
}

.tab-item.active {
    color: var(--app-accent);
}
</style>
