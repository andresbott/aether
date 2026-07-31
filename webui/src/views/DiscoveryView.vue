<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import DiscoveryFeedItem from '@/components/library/DiscoveryFeedItem.vue'
import { useDiscoveryFeed } from '@/composables/useDiscovery'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const layout = computed<Layout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ query })
    }
})

const { items, isLoading, isError, hasNextPage, isFetchingNextPage, fetchNextPage, refresh } =
    useDiscoveryFeed()

const summary = computed(() => {
    const n = items.value.length
    if (n === 0) return ''
    return `${n} item${n === 1 ? '' : 's'}`
})

const isEmpty = computed(() => !isLoading.value && !isError.value && items.value.length === 0)

// Intersection observer over a sentinel at the end of the feed. Attached only
// while more pages remain, so a fully-loaded feed holds no observer.
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const stopObserving = (): void => {
    observer?.disconnect()
    observer = null
}

watch(sentinel, (el) => {
    stopObserving()
    if (!el || typeof IntersectionObserver === 'undefined') return
    observer = new IntersectionObserver((entries) => {
        if (entries.some((e) => e.isIntersecting)) fetchNextPage()
    })
    observer.observe(el)
})

onBeforeUnmount(stopObserving)
</script>

<template>
    <ContentScaffold title="Discovery" :summary="summary">
        <template #actions>
            <Button
                class="discovery-refresh"
                icon="pi pi-refresh"
                text
                rounded
                aria-label="Refresh"
                @click="refresh"
            />
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
        </template>

        <div class="discovery-scroll">
            <div v-if="isLoading" class="discovery-state content-col">
                <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
            </div>

            <div v-else-if="isError" class="discovery-state content-col">
                <i class="pi pi-exclamation-triangle"></i>
                <span>Could not load the discovery feed</span>
            </div>

            <div v-else-if="isEmpty" class="discovery-state content-col">
                <span>Nothing here yet</span>
            </div>

            <template v-else>
                <div class="discovery-feed content-col" :class="layout">
                    <DiscoveryFeedItem
                        v-for="entry in items"
                        :key="`${entry.type}-${entry.rank}`"
                        :entry="entry"
                        :layout="layout"
                    />
                </div>

                <div v-if="hasNextPage" ref="sentinel" class="discovery-sentinel content-col">
                    <i
                        v-if="isFetchingNextPage"
                        class="pi pi-spin pi-spinner"
                        style="font-size: 1.25rem"
                    ></i>
                </div>
            </template>
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* Recipe B (see docs/architecture/main-content-view-layout.md): plain scrolling
   body, one scrollbar's worth of rail clearance on the right. --sb-w comes from
   PlayerLayout; never re-measure it here. */
.discovery-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-bottom: 1rem;
    box-sizing: border-box;
}

.discovery-feed.grid {
    padding-top: 1rem;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 1.5rem;
}

.discovery-feed.list {
    padding-top: 1rem;
    display: flex;
    flex-direction: column;
}

.discovery-state {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding-block: 1.5rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}

.discovery-sentinel {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 3rem;
    color: var(--app-text-secondary);
}
</style>
