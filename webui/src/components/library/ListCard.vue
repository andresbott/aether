<script lang="ts">
/** One fact on a card's lines of facts, with an icon after its text. */
export interface ListCardMetaPart {
    text: string
    // A literal `ms-*` class in the caller's source, where the icon build finds it.
    icon?: string
    // Names the icon for screen readers, and as its tooltip.
    iconLabel?: string
}
</script>

<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { useListCardMetrics } from '@/composables/useListCardMetrics'

/**
 * One item of a library list in the list layout: the cover large on the left with
 * the item's identity stacked beside it — the detail hero's hierarchy
 * (`HeroHeader`) brought down to list scale. Like the grid cards, it shows no
 * chrome until hovered. Every entity row renders through it (`AlbumRow`,
 * `ArtistRow`, `PlaylistRow`, `GenreRow`, `RadioStationRow`), so the lists share
 * one size, one look and one heart.
 *
 * Without `to` it renders the placeholder a lazily loaded slot shows until its
 * page arrives, at the same size, so a virtual list never jumps.
 */
const props = withDefaults(
    defineProps<{
        to?: RouteLocationRaw
        title?: string
        subtitle?: string
        // The facts under the subtitle, joined like the hero's meta row: `meta` a
        // bold first line with the item's key facts ("2004", "12 songs", "43 min"),
        // `details` a lighter second line with what it is and its history ("EP",
        // "Latin Pop"; a playlist's owner and dates). Each runs most important
        // first: a line too long for the card is cut at its end.
        meta?: Array<string | ListCardMetaPart>
        details?: Array<string | ListCardMetaPart>
        coverUrl?: string | null
        placeholderIcon?: string
        // Artists and stations: a round cover, as on the artist hero.
        round?: boolean
        // Renders the favorite toggle; `starred` is its state.
        starrable?: boolean
        starred?: boolean
    }>(),
    {
        to: undefined,
        title: '',
        subtitle: '',
        meta: () => [],
        details: () => [],
        coverUrl: null,
        placeholderIcon: 'ms-music-note',
        round: false,
        starrable: false,
        starred: false
    }
)

const emit = defineEmits<{ (e: 'star'): void }>()

const { metrics } = useListCardMetrics()
const sizeVars = computed(() => ({
    '--list-card-cover': `${metrics.value.cover}px`,
    '--list-card-pad': `${metrics.value.pad}px`,
    '--list-card-gap': `${metrics.value.gap}px`
}))

// An empty part is a fact the item lacks; it is dropped with its separator, and
// a line left with no parts is dropped whole.
const toParts = (line: Array<string | ListCardMetaPart>): ListCardMetaPart[] =>
    line
        .map((part): ListCardMetaPart => (typeof part === 'string' ? { text: part } : part))
        .filter((part) => part.text !== '')

const factLines = computed(() =>
    [
        { name: 'card-meta', parts: toParts(props.meta) },
        { name: 'card-details', parts: toParts(props.details) }
    ].filter((line) => line.parts.length > 0)
)

// The card is a router-link, so the heart swallows its click; otherwise it would
// open the item instead of toggling the favorite.
const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    emit('star')
}
</script>

<template>
    <div
        v-if="!to"
        class="list-card placeholder"
        :class="{ round }"
        :style="sizeVars"
        aria-hidden="true"
    >
        <div class="card-cover"></div>
        <div class="card-identity">
            <span class="skeleton title-skeleton"></span>
            <span class="skeleton meta-skeleton"></span>
        </div>
    </div>
    <router-link v-else :to="to" class="list-card" :class="{ round }" :style="sizeVars">
        <!-- Unlabelled: the title beside it already names the item. Not
             draggable, so grabbing the cover drags whatever the host made the
             card carry rather than the browser's image. -->
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" alt="" draggable="false" />
            <div v-else class="cover-placeholder"><i :class="placeholderIcon"></i></div>
        </div>
        <div class="card-identity">
            <div class="card-title">{{ title }}</div>
            <div v-if="subtitle" class="card-subtitle">{{ subtitle }}</div>
            <div v-if="factLines.length" class="card-facts">
                <div
                    v-for="line in factLines"
                    :key="line.name"
                    class="card-line"
                    :class="line.name"
                >
                    <span v-for="(part, i) in line.parts" :key="i" class="meta-part">
                        <span>{{ part.text }}</span>
                        <i
                            v-if="part.icon"
                            class="meta-icon"
                            :class="part.icon"
                            role="img"
                            :aria-label="part.iconLabel"
                            :title="part.iconLabel"
                        ></i>
                    </span>
                </div>
            </div>
        </div>
        <button
            v-if="starrable"
            class="row-star"
            :class="{ 'is-starred': starred }"
            type="button"
            :aria-label="starred ? 'Remove from favorites' : 'Add to favorites'"
            @click="onStar"
        >
            <i :class="starred ? 'ms-favorite' : 'mso-favorite'"></i>
        </button>
    </router-link>
</template>

<style scoped>
/* Height + bottom margin = the virtual list's itemSize (useListCardMetrics), so
   the card fills its slot exactly. The width rules are Recipe C's row centering
   (main-content-view-layout.md §4); `width: 100%` keeps the auto margins from
   shrinking the card to its content inside a flex column (Search, Discover). */
.list-card {
    display: grid;
    grid-template-columns: var(--list-card-cover) minmax(0, 1fr) auto;
    align-items: center;
    column-gap: 1.25rem;
    box-sizing: border-box;
    width: 100%;
    max-width: var(--app-content-max-width);
    height: calc(var(--list-card-cover) + 2 * var(--list-card-pad));
    margin: 0 auto var(--list-card-gap);
    padding: var(--list-card-pad);
    border-radius: var(--app-radius);
    color: var(--app-text-primary);
    text-decoration: none;
    transition:
        background-color 0.2s,
        box-shadow 0.2s;
}

/* No chrome at rest, like the grid cards: hover brings up their accent outline,
   tint and lift (AlbumCard and its siblings). The outline is an inset shadow
   rather than their border, so it takes nothing from the slot's height. */
.list-card:not(.placeholder):hover {
    background-color: var(--app-accent-soft);
    box-shadow:
        inset 0 0 0 1px var(--app-accent),
        0 6px 18px rgba(0, 0, 0, 0.12);
}

/* --- Cover ---------------------------------------------------------------- */
.card-cover {
    width: var(--list-card-cover);
    height: var(--list-card-cover);
    border-radius: 4px;
    overflow: hidden;
    background: var(--app-bg-subtle);
}

.list-card.round .card-cover {
    border-radius: 50%;
}

.card-cover img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.85);
    font-size: calc(var(--list-card-cover) * 0.3);
}

/* --- Identity: the hero's hierarchy at list scale --------------------------- */
.card-identity {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
}

.card-title,
.card-subtitle,
.card-line {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.card-title {
    font-size: 1.2rem;
    font-weight: 800;
    letter-spacing: -0.015em;
    line-height: 1.25;
}

/* The hero's cyan artist line. */
.card-subtitle {
    font-size: 0.9375rem;
    font-weight: 500;
    line-height: 1.3;
    color: var(--app-list-card-subtitle);
}

/* The two lines of facts sit a little apart from the identity above and tight
   together: title + subtitle + both lines stay within the cover's height
   (useListCardMetrics), so a card with every line filled still centers on its cover. */
.card-facts {
    margin-top: 0.2rem;
}

.card-line {
    font-size: 0.8125rem;
    font-weight: 400;
    line-height: 1.3;
    color: var(--app-list-card-meta);
    font-variant-numeric: tabular-nums;
}

/* The key facts in bold, over the lighter details. */
.card-meta {
    font-weight: 600;
}

.meta-part + .meta-part::before {
    content: '•';
    margin: 0 0.5em;
    opacity: 0.6;
}

.meta-icon {
    margin-left: 0.4em;
    font-size: 0.95em;
}

/* --- Favorite --------------------------------------------------------------
   Hover-revealed, pinned visible while favorited, grey-but-filled rather than
   accented — the rules every heart in the app follows (see
   unified-play-experience.md). Touch has no hover, so it is always shown there. */
.row-star {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.25rem;
    height: 2.25rem;
    border: none;
    border-radius: 50%;
    background: none;
    padding: 0;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 1.1rem;
    cursor: pointer;
    opacity: 0;
    transition:
        opacity 0.15s,
        color 0.15s,
        background-color 0.15s;
}

.list-card:hover .row-star,
.list-card:focus-visible .row-star,
.row-star:focus-visible,
.row-star.is-starred {
    opacity: 1;
}

.row-star:hover,
.row-star:focus-visible {
    color: var(--app-text-primary);
    background: color-mix(in srgb, var(--app-text-primary) 10%, transparent);
}

@media (pointer: coarse) {
    .row-star {
        opacity: 1;
    }
}

/* --- Placeholder: the card's shape, still ---------------------------------- */
.list-card.placeholder .card-cover,
.skeleton {
    background: color-mix(in srgb, var(--app-text-secondary) 16%, transparent);
}

.skeleton {
    display: block;
    height: 0.75rem;
    border-radius: 999px;
}

.title-skeleton {
    width: min(16rem, 60%);
    height: 1rem;
}

.meta-skeleton {
    width: min(9rem, 35%);
    margin-top: 0.5rem;
}

/* Phone: a smaller card (useListCardMetrics) with type to match.
   767.98px = $bp-phone-max - 0.02px (breakpoints.spec.ts guards the token). */
@media (max-width: 767.98px) {
    .list-card {
        column-gap: 0.875rem;
    }

    .card-title {
        font-size: 1.0625rem;
    }

    .card-subtitle {
        font-size: 0.875rem;
    }

    .card-line {
        font-size: 0.75rem;
    }
}

@media (prefers-reduced-motion: reduce) {
    .list-card,
    .row-star {
        transition: none;
    }
}
</style>
