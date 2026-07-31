<script setup lang="ts" generic="T extends { id: string }">
import { computed, onBeforeUnmount, onMounted, ref, watch, nextTick } from 'vue'
import VirtualScroller from 'primevue/virtualscroller'
import type { VirtualScrollerLazyEvent } from 'primevue/virtualscroller'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import CardGrid from '@/components/library/CardGrid.vue'
import type { AlbumLetter } from '@/types/subsonic'
import {
    chunkRows,
    computeColumnWidth,
    computeGridColumns,
    offsetToRow,
    rowRangeToItemRange
} from '@/utils/cardGrid'

const props = withDefaults(
    defineProps<{
        items: (T | undefined)[]
        letters: AlbumLetter[]
        total: number
        minColWidth?: number
        gap?: number
        pageSize?: number
        showRail?: boolean
    }>(),
    { minColWidth: 200, gap: 32, pageSize: 100, showRail: true }
)

const emit = defineEmits<{ (e: 'lazyLoad', first: number, last: number): void }>()
defineSlots<{ card(props: { item: T | undefined }): unknown }>()

const INFO_HEIGHT_ESTIMATE = 56

const scroller = ref<InstanceType<typeof VirtualScroller> | null>(null)
const gridRoot = ref<HTMLElement | null>(null)
const sizer = ref<HTMLElement | null>(null)

const availableWidth = ref(0)
const infoHeight = ref(INFO_HEIGHT_ESTIMATE)

const columns = computed(() => computeGridColumns(availableWidth.value, props.minColWidth, props.gap))
const columnWidth = computed(() => computeColumnWidth(availableWidth.value, columns.value, props.gap))
const itemSize = computed(() => Math.max(1, columnWidth.value + infoHeight.value + props.gap))
const rows = computed(() => chunkRows(props.items, columns.value))
const rowStyle = computed(() => ({
    // VirtualScroller lays rows out in normal flow and only uses itemSize for the
    // scroll spacer/offset, so rows would otherwise stack flush (no vertical gap).
    // A bottom margin of `gap` gives the vertical spacing AND makes each row's flow
    // height equal itemSize (columnWidth + infoHeight + gap), avoiding scroll drift.
    // getBoundingClientRect (used to measure info height) excludes margins, so this
    // doesn't feed back into the sizing.
    marginBottom: `${props.gap}px`
}))

let lastRowRange: { first: number; last: number } | null = null

function emitItemRange(firstRow: number, lastRow: number): void {
    if (props.total <= 0) return
    const r = rowRangeToItemRange(firstRow, lastRow, columns.value, props.total)
    emit('lazyLoad', r.first, r.last)
}

function kickInitial(): void {
    if (props.total > 0) emit('lazyLoad', 0, Math.min(props.total - 1, props.pageSize - 1))
}

function measureWidth(): void {
    const w = sizer.value?.clientWidth ?? 0
    if (w > 0) availableWidth.value = w
}

// Refine the (font-based, width-independent) info height from a real row so rows never overlap.
// Cards whose caption overlays the cover (e.g. albums) have no info below the square, so the
// measured height equals the column width — clamp to 0 rather than keeping the estimate.
function measureInfoHeight(): void {
    const rowEl = gridRoot.value?.querySelector('.grid-row') as HTMLElement | null
    if (!rowEl || columnWidth.value <= 0) return
    const info = rowEl.getBoundingClientRect().height - columnWidth.value
    infoHeight.value = Math.max(0, info)
}

function onLazyLoad(e: VirtualScrollerLazyEvent): void {
    lastRowRange = { first: e.first, last: e.last }
    emitItemRange(e.first, e.last)
}

function onSelect(offset: number): void {
    if (props.total > 0) emit('lazyLoad', offset, Math.min(props.total - 1, offset + props.pageSize - 1))
    scroller.value?.scrollToIndex(offsetToRow(offset, columns.value))
    // Update remembered range so a resize immediately after a rail jump re-emits the jumped range, not the stale pre-jump one.
    lastRowRange = { first: offsetToRow(offset, columns.value), last: offsetToRow(Math.min(props.total - 1, offset + props.pageSize - 1), columns.value) }
}

let ro: ResizeObserver | null = null

onMounted(() => {
    measureWidth()
    kickInitial()
    void nextTick(measureInfoHeight)
    if (typeof ResizeObserver !== 'undefined' && sizer.value) {
        ro = new ResizeObserver(() => measureWidth())
        ro.observe(sizer.value)
    }
})

onBeforeUnmount(() => ro?.disconnect())

// Re-emit for the current viewport when the column count changes on resize.
watch(columns, () => {
    if (lastRowRange) emitItemRange(lastRowRange.first, lastRowRange.last)
})

// Reload page 0 when the dataset resets (folder switch / total change).
watch(
    () => props.total,
    () => {
        lastRowRange = null
        kickInitial()
    }
)

// Refine the row height once the first real item lands.
watch(
    () => props.items[0],
    (v) => {
        if (v) void nextTick(measureInfoHeight)
    }
)
</script>

<template>
    <div ref="gridRoot" class="card-grid">
        <div class="grid-sizer" aria-hidden="true">
            <div ref="sizer" class="grid-sizer-inner"></div>
        </div>
        <VirtualScroller
            ref="scroller"
            :items="rows"
            :itemSize="itemSize"
            lazy
            :numToleratedItems="2"
            class="grid-scroller"
            @lazy-load="onLazyLoad"
        >
            <template #item="{ item: row }">
                <!-- One CardGrid per virtual row: the shared layout owns the column
                     math and the min-width:0 constraint, this component only adds
                     virtualization, row spacing and the alphabet rail on top. -->
                <CardGrid
                    class="grid-row"
                    :style="rowStyle"
                    :items="(row as (T | undefined)[])"
                    :minColWidth="minColWidth"
                    :gap="gap"
                >
                    <template #card="{ item }">
                        <slot name="card" :item="item as T | undefined" />
                    </template>
                </CardGrid>
            </template>
        </VirtualScroller>
        <AlphabetRail v-if="showRail" :letters="letters" @select="onSelect" />
    </div>
</template>

<style scoped>
.card-grid {
    position: relative;
    height: 100%;
    min-height: 0;
}

/* Hidden probe whose inner width equals a row's inner width (max-width capped, centered,
   with the same right clearance the scroller content reserves for rail + scrollbar). */
.grid-sizer {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 0;
    overflow: hidden;
    pointer-events: none;
    box-sizing: border-box;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px) + var(--app-content-gutter));
}

.grid-sizer-inner {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
}

.grid-scroller {
    height: 100%;
    width: 100%;
    scrollbar-gutter: stable;
}

/* Recipe C: rail clearance + shared gutter on the scroll content so centered
   rows never slide under the rail; border-box keeps the min-width:100% content
   wrapper from overflowing. */
.card-grid :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px) + var(--app-content-gutter));
}

/* Center each row in the shared content column; the scroller stays full width so its
   scrollbar and the rail stay flush right. */
.grid-row {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
    box-sizing: border-box;
}

/* Rail hugs the LEFT of the flush-right native scrollbar (offset by its width). */
.card-grid :deep(.alphabet-rail) {
    position: absolute;
    top: 0;
    bottom: 0;
    right: var(--sb-w, 0px);
    width: 1.75rem;
    background: var(--app-bg, transparent);
}
</style>
