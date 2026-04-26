<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import SearchBar from '@/components/layout/SearchBar.vue'
import UserMenu from '@/components/layout/UserMenu.vue'

const searchOpen = ref(false)

function openSearch() {
    searchOpen.value = true
}

function closeSearch() {
    searchOpen.value = false
}

function onSearchBlur() {
    // Delay so clicks on the close button or search results land before we collapse.
    setTimeout(closeSearch, 200)
}
</script>

<template>
    <header class="app-topbar" :class="{ 'search-open': searchOpen }">
        <div class="topbar-left">
            <RouterLink to="/" class="logo-link">
                <h1 class="logo">Aether</h1>
            </RouterLink>
        </div>

        <div class="topbar-center">
            <div id="topbar-search" class="search-wrapper" @focusout="onSearchBlur">
                <SearchBar />
            </div>
            <button
                class="search-icon-btn"
                type="button"
                aria-label="Open search"
                :aria-expanded="searchOpen"
                aria-controls="topbar-search"
                @click="openSearch"
            >
                <i class="pi pi-search"></i>
            </button>
        </div>

        <div class="topbar-right">
            <div class="user-menu-slot">
                <UserMenu />
            </div>
            <button
                class="search-close-btn"
                type="button"
                aria-label="Close search"
                @click="closeSearch"
            >
                <i class="pi pi-times"></i>
            </button>
        </div>
    </header>
</template>

<style scoped>
.app-topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.5rem 1.5rem;
    height: 64px;
    background-color: var(--app-surface);
    border-bottom: 1px solid var(--app-border);
    flex-shrink: 0;
}

.topbar-left {
    display: flex;
    align-items: center;
    flex-shrink: 0;
}

.logo-link {
    text-decoration: none;
}

.logo {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--app-accent);
    margin: 0;
    white-space: nowrap;
}

.topbar-center {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    min-width: 0;
}

.search-wrapper {
    width: 100%;
    max-width: 320px;
}

.search-icon-btn,
.search-close-btn {
    display: none;
    background: none;
    border: none;
    cursor: pointer;
    padding: 0.5rem;
    border-radius: 6px;
    color: var(--app-text-secondary);
    transition: background-color 0.15s;
}

.search-icon-btn:hover,
.search-close-btn:hover {
    background-color: var(--app-background);
}

.topbar-right {
    display: flex;
    align-items: center;
    flex-shrink: 0;
}

@media (max-width: 768px) {
    .app-topbar {
        padding: 0.5rem 1rem;
    }

    .search-wrapper {
        display: none;
    }

    .search-icon-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        margin-left: auto;
    }

    .app-topbar.search-open .topbar-left,
    .app-topbar.search-open .search-icon-btn,
    .app-topbar.search-open .user-menu-slot {
        display: none;
    }

    .app-topbar.search-open .search-wrapper {
        display: block;
        max-width: none;
    }

    .app-topbar.search-open .search-close-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
    }
}
</style>
