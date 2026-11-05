<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import VirtualScroller from 'primevue/virtualscroller'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import WishlistRow from '@/components/wishlist/WishlistRow.vue'
import { useWishlist } from '@/composables/useWishlist'
import { useListCardMetrics } from '@/composables/useListCardMetrics'
import { WISHLIST_KINDS, isWishlistKind } from '@/lib/wishlist'
import type { WishlistKind } from '@/types/wishlist'

type KindFilter = 'all' | WishlistKind

const route = useRoute()
const router = useRouter()

const { data: items, isLoading, error } = useWishlist()
const { itemSize } = useListCardMetrics()

const kindOptions = [
    { label: 'All', value: 'all' },
    ...WISHLIST_KINDS.map((k) => ({ label: k.plural, value: k.value }))
]

// The kind filter is URL state (`?kind=album`), like the library's layout, so a
// reload or a shared link keeps it.
const kind = computed<KindFilter>({
    get: () => (isWishlistKind(route.query.kind) ? route.query.kind : 'all'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'all') delete query.kind
        else query.kind = v
        router.replace({ query })
    }
})

const visible = computed(() => {
    const all = items.value ?? []
    return kind.value === 'all' ? all : all.filter((it) => it.kind === kind.value)
})

const summary = computed(() => {
    const count = visible.value.length
    if (count === 0) return ''
    const found = visible.value.filter((it) => it.in_library).length
    const base = `${count} ${count === 1 ? 'item' : 'items'}`
    return found > 0 ? `${base} · ${found} in library` : base
})

const emptyText = computed(() =>
    kind.value === 'all'
        ? 'Your wishlist is empty'
        : `No ${WISHLIST_KINDS.find((k) => k.value === kind.value)?.plural.toLowerCase()} on your wishlist`
)

function openAdd() {
    router.push({ name: 'wishlist-new', query: kind.value === 'all' ? {} : { kind: kind.value } })
}
</script>

<template>
    <ContentScaffold title="Wishlist" :summary="summary">
        <template #actions>
            <SelectButton
                v-model="kind"
                :options="kindOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Show"
                class="as-button-group wishlist-kind-filter"
            />
            <Button
                class="add-wishlist-item"
                icon="ms-add"
                text
                rounded
                v-tooltip.bottom="'Add to wishlist'"
                aria-label="Add to wishlist"
                @click="openAdd"
            />
        </template>

        <div class="wishlist-list-view">
            <div v-if="isLoading" class="loading">
                <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
            </div>
            <div v-else-if="error" class="empty-state">
                <i class="ms-warning" style="font-size: 3rem"></i>
                <p>Could not load the wishlist</p>
            </div>
            <div v-else-if="visible.length === 0" class="empty-state">
                <i class="ms-bookmark-heart" style="font-size: 3rem"></i>
                <p class="empty-title">{{ emptyText }}</p>
                <p v-if="kind === 'all'" class="empty-hint">
                    Search MusicBrainz for an artist, album or song you want. When a scan brings it into
                    your library, it is marked as found here.
                </p>
                <Button label="Add to wishlist" icon="ms-add" outlined @click="openAdd" />
            </div>
            <div v-else class="list-body">
                <VirtualScroller :items="visible" :itemSize="itemSize" class="list-scroller">
                    <template #item="{ item }">
                        <WishlistRow :item="item" />
                    </template>
                </VirtualScroller>
            </div>
        </div>
    </ContentScaffold>
</template>

<style scoped>
.wishlist-list-view {
    height: 100%;
    min-height: 0;
}

.list-body {
    position: relative;
    height: 100%;
}

.list-scroller {
    height: 100%;
    width: 100%;
    scrollbar-gutter: stable;
}

/* Recipe C without a rail, as the radio list: the shared gutter on both sides;
   the top padding is the gap under the header. */
.list-body :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-top: 1rem;
    padding-inline: var(--app-content-gutter);
}

.loading {
    display: flex;
    justify-content: center;
    padding: 3rem;
    color: var(--app-text-secondary);
}

.empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 4rem 1rem;
    gap: 1rem;
    color: var(--app-text-secondary);
    text-align: center;
}
.empty-state p {
    margin: 0;
}
.empty-title {
    font-weight: 600;
    color: var(--app-text-primary);
}
.empty-hint {
    max-width: 28rem;
    line-height: 1.5;
}
</style>
