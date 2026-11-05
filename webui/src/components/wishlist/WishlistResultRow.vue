<script setup lang="ts">
import { computed } from 'vue'
import { wishlistDetails, wishlistKind, wishlistLibraryRoute, wishlistSubtitle } from '@/lib/wishlist'
import type { WishlistSearchResult } from '@/types/wishlist'

/**
 * One MusicBrainz result in the wishlist search: a pickable row (a checkbox
 * button, in the app's `mso-circle` / `ms-check-circle` selection idiom) with
 * the result's identity, and its status beside it — "On wishlist" (not
 * pickable: it is already listed) and "In library" (a link to it there).
 */
const props = defineProps<{ result: WishlistSearchResult; selected: boolean }>()
const emit = defineEmits<{ (e: 'toggle'): void }>()

const listed = computed(() => !!props.result.wishlist_id)
const libraryRoute = computed(() => wishlistLibraryRoute(props.result))
const subtitle = computed(() => wishlistSubtitle(props.result))
const details = computed(() => wishlistDetails(props.result).join(' · '))
const statusId = computed(() => `wishlist-status-${props.result.kind}-${props.result.mbid}`)
</script>

<template>
    <div class="result-row" :class="{ selected, listed }">
        <button
            type="button"
            class="result-pick"
            role="checkbox"
            :aria-checked="listed || selected"
            :disabled="listed"
            :aria-describedby="listed || libraryRoute ? statusId : undefined"
            @click="emit('toggle')"
        >
            <i class="pick-icon" :class="listed || selected ? 'ms-check-circle' : 'mso-circle'" aria-hidden="true"></i>
            <span class="result-cover" :class="{ round: result.kind === 'artist' }">
                <i :class="wishlistKind(result.kind).icon" aria-hidden="true"></i>
            </span>
            <span class="result-body">
                <span class="result-title">{{ result.title }}</span>
                <span v-if="subtitle" class="result-subtitle">{{ subtitle }}</span>
                <span v-if="details" class="result-details">{{ details }}</span>
            </span>
        </button>
        <span :id="statusId" class="result-badges">
            <span v-if="listed" class="badge listed-badge">On wishlist</span>
            <router-link v-if="libraryRoute" :to="libraryRoute" class="badge library-badge">
                <i class="ms-check-circle" aria-hidden="true"></i> In library
            </router-link>
        </span>
    </div>
</template>

<style scoped>
.result-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    border-radius: 8px;
    padding-right: 0.75rem;
}
.result-row:hover:not(.listed) {
    background: var(--app-bg-subtle);
}
.result-row.selected {
    background: var(--app-accent-soft);
}
.result-pick {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.9rem;
    padding: 0.5rem 0.75rem;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
}
.result-pick:disabled {
    cursor: default;
}
.result-pick:focus-visible {
    outline: 2px solid var(--app-accent);
    outline-offset: -2px;
    border-radius: 8px;
}
.pick-icon {
    font-size: 1.3rem;
    color: var(--app-text-secondary);
    flex-shrink: 0;
}
.selected .pick-icon {
    color: var(--app-accent);
}
.listed .pick-icon {
    opacity: 0.5;
}
/* A cover-sized tile: MusicBrainz results have no art of their own here. */
.result-cover {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 6px;
    background: var(--app-bg-subtle);
    color: var(--app-text-secondary);
    font-size: 1.4rem;
    flex-shrink: 0;
}
.result-cover.round {
    border-radius: 50%;
}
.result-body {
    display: flex;
    flex-direction: column;
    min-width: 0;
}
.result-title {
    font-weight: 600;
}
.result-subtitle,
.result-details {
    color: var(--app-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.result-subtitle {
    font-size: 0.875rem;
}
.result-details {
    font-size: 0.8rem;
}
.result-badges {
    display: flex;
    gap: 0.4rem;
    flex-shrink: 0;
}
.badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.15rem 0.5rem;
    border-radius: 999px;
    border: 1px solid var(--app-border);
    color: var(--app-text-secondary);
    white-space: nowrap;
}
.library-badge {
    color: var(--app-accent);
    border-color: currentColor;
    text-decoration: none;
}
.library-badge:hover {
    background: var(--app-accent-soft);
}
</style>
