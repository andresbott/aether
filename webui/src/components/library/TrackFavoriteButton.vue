<script setup lang="ts">
import { useSongFavorite } from '@/composables/useSongFavorite'
import type { Song } from '@/types/subsonic'

/**
 * The favorite toggle for one track row — the row counterpart to the cards'
 * `.card-star` and the hero's `.hero-action-star`. Every track row in the app
 * renders this rather than repeating the icon pair, the wording and the
 * click-swallowing (`AlbumTrackRow`, `GenreTrackRow`, `QueueRow`).
 *
 * Its root element keeps the `.row-star` class `PlaylistRow` established, so a
 * host row owns the reveal rule (`.<row>:hover .row-star { opacity: 1 }`) the
 * same way — hover semantics differ per row, the icon and wording do not.
 *
 * The click is both `preventDefault`ed and `stopPropagation`ed: rows are
 * click-to-select and sometimes wrapped in a link, so an unhandled click would
 * select the row or navigate away instead of starring.
 */
const props = defineProps<{ song?: Song }>()

const { isStarred, toggleFavorite } = useSongFavorite(() => props.song)

const onClick = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    toggleFavorite()
}
</script>

<template>
    <button
        class="row-star"
        :class="{ 'is-starred': isStarred }"
        type="button"
        :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
        @click="onClick"
        @dblclick.stop.prevent
    >
        <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
    </button>
</template>

<style scoped>
/*
 * Hidden until the row is hovered — except when the track IS a favorite, where
 * it stays visible so a list reads as a set of favorites at a glance. The
 * reveal-on-hover half lives in each host row (it needs that row's class);
 * everything that must look the same everywhere lives here.
 */
.row-star {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 0;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 1rem;
    cursor: pointer;
    opacity: 0;
    transition:
        opacity 0.15s,
        color 0.15s;
}

/* A favorite reads as favorite by the FILL alone, not by colour — the accent is
   reserved for what is playing and for interactive affordances, and a list of
   accent-coloured hearts competed with it. Same rule for every heart in the app;
   see unified-play-experience.md. */
.row-star.is-starred {
    opacity: 1;
}

.row-star:hover,
.row-star:focus-visible {
    opacity: 1;
    color: var(--app-text-primary);
}

/* Touch: the heart is a primary, always-visible affordance sitting one grid
   gap from the ⋮, and a missed tap lands on the row — which REPLACES the
   queue. Give it the host rows' full 2rem column and stretch the hit area to
   ~44px with an invisible overlay; growing the element itself would overflow
   the hosts' pinned 2rem grid tracks. */
@media (pointer: coarse) {
    .row-star {
        position: relative;
        width: 2rem;
        height: 2rem;
    }

    .row-star::after {
        content: '';
        position: absolute;
        inset: -6px;
    }
}
</style>
