<script setup lang="ts" generic="T extends { key: string }">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

/**
 * One section of the mobile landing page (`MobileBrowseView`): a heading, a few
 * items in a horizontally swipeable strip, and a "See all" link to the full view
 * the section samples.
 *
 * Presentational only — every consumer owns its own query and hands over items it
 * has already truncated to `BROWSE_SHELF_SIZE`, so the shelf never has to know
 * whether it is showing albums, playlists, genres or stations (same division of
 * labour as `CardGrid`, which owns the grid layout and nothing else).
 *
 * Items carry an explicit `key` rather than the `id` `CardGrid` keys on: genres
 * have no id — their identity is `value` — so the consumer names the key.
 */
const props = withDefaults(
    defineProps<{
        title: string
        to: RouteLocationRaw
        items: T[]
        icon?: string
        loading?: boolean
        error?: boolean
        errorText?: string
        emptyText?: string
    }>(),
    { errorText: 'Could not load this section', emptyText: 'Nothing here yet' }
)

defineSlots<{ card(props: { item: T }): unknown }>()

// An empty section still renders its heading and link: "See all" is the way to
// the view that can fix the emptiness (create a playlist, add a station).
const isEmpty = computed(() => !props.loading && !props.error && props.items.length === 0)
</script>

<template>
    <section class="browse-shelf">
        <header class="shelf-header">
            <h2 class="shelf-title">
                <i v-if="icon" :class="icon" aria-hidden="true"></i>
                <span>{{ title }}</span>
            </h2>
            <!-- Labelled per section, so a screen reader hears which "See all"
                 this is when the links are read out of context. -->
            <router-link class="shelf-more" :to="to" :aria-label="`See all in ${title}`">
                <span>See all</span>
                <i class="pi pi-angle-right" aria-hidden="true"></i>
            </router-link>
        </header>

        <div v-if="loading" class="shelf-state">
            <i class="pi pi-spin pi-spinner" aria-hidden="true"></i>
        </div>

        <!-- Distinct from the empty branch on purpose: a failed request must not
             read as "this section has nothing in it". -->
        <div v-else-if="error" class="shelf-state">
            <i class="pi pi-exclamation-triangle" aria-hidden="true"></i>
            <span>{{ errorText }}</span>
        </div>

        <p v-else-if="isEmpty" class="shelf-state">{{ emptyText }}</p>

        <ul v-else class="shelf-strip">
            <li v-for="item in items" :key="item.key" class="shelf-item">
                <slot name="card" :item="item" />
            </li>
        </ul>
    </section>
</template>

<style scoped>
.browse-shelf {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    padding-top: 1.25rem;
}

.shelf-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
}

.shelf-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
    margin: 0;
    font-size: 1.05rem;
    font-weight: 700;
}

.shelf-title span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* The section's nav target. Padded rather than sized so it clears a thumb
   without stretching the heading row. */
.shelf-more {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 0.15rem;
    padding: 0.4rem 0.15rem;
    color: var(--app-accent);
    font-size: 0.85rem;
    font-weight: 600;
    text-decoration: none;
}

.shelf-strip {
    display: flex;
    gap: 0.75rem;
    margin: 0;
    padding: 0;
    list-style: none;
    overflow-x: auto;
    /* The strip is a horizontal gesture surface inside a vertically scrolling
       page: contain the overscroll so a sideways fling never chains out into the
       browser's back-swipe, and snap so a released swipe lands on a card. */
    overscroll-behavior-x: contain;
    scroll-snap-type: x proximity;
}

.shelf-item {
    /* Fixed width, so a card's title cannot size the strip's tracks unevenly and
       every cover stays square. Roughly matches the phone column cap CardGrid
       applies (150px), which keeps cards the same size as on /library. */
    flex: 0 0 8.5rem;
    min-width: 0;
    scroll-snap-align: start;
}

/* The card itself is slotted content, so it needs :deep to be reached from here.
   A card is a flex container whose own min-width resolves to min-content as a
   layout child — without this it sizes to its longest unbreakable title and
   overflows its track, which is what makes one card render bigger than its
   neighbours (same constraint DiscoveryFeedItem applies inside CardGrid). */
.shelf-item > :deep(*) {
    min-width: 0;
    max-width: 100%;
}

.shelf-state {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 3rem;
    margin: 0;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}
</style>
