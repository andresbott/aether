<script setup lang="ts" generic="T extends { id: string }">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { computeColumnWidth, computeGridColumns } from '@/utils/cardGrid'
import { useViewport } from '@/composables/useViewport'

/**
 * The single responsive card-grid LAYOUT in the app: it measures the available
 * width, derives the column count, and renders one uniform track per column.
 *
 * It owns no scrolling and no paging, which is what lets the two very different
 * consumers share it:
 *   - `VirtualCardGrid` wraps it per row inside a VirtualScroller for
 *     index-addressed lists (`/library`: known total, alphabet rail, jump to "S").
 *   - `DiscoveryFeed` renders it once for the whole cursor-paged feed (no total,
 *     no rail, an IntersectionObserver appends pages).
 *
 * Keeping the column math here is not cosmetic. `grid-template-columns` with a
 * `1fr` max is really `minmax(auto, 1fr)`, so a track's AUTO minimum is its
 * content's intrinsic width: one long card title stretches its own column wider
 * than the others and scales that card's square cover with it. Cards truncate with
 * `text-overflow: ellipsis`, but that only engages once the box is constrained.
 * `minmax(0, 1fr)` plus `min-width: 0` on the cells is the constraint, and it lives
 * in exactly one place so no consumer can get it wrong.
 */
const props = withDefaults(
    defineProps<{
        items: (T | undefined)[]
        minColWidth?: number
        gap?: number
    }>(),
    { minColWidth: 200, gap: 32 }
)

defineSlots<{ card(props: { item: T | undefined }): unknown }>()

const { tier } = useViewport()

// Phones get tighter cards: a 200px minimum drops a 390px viewport to a single
// monster column. Callers' explicit smaller values are respected, and the caps are
// idempotent — `VirtualCardGrid` applies the same ones before passing them down,
// while `DiscoveryFeed` renders this component directly and relies on these.
const effectiveMinColWidth = computed(() =>
    tier.value === 'phone' ? Math.min(props.minColWidth, 150) : props.minColWidth
)
const effectiveGap = computed(() => (tier.value === 'phone' ? Math.min(props.gap, 16) : props.gap))

const sizer = ref<HTMLElement | null>(null)
const availableWidth = ref(0)

const columns = computed(() =>
    computeGridColumns(availableWidth.value, effectiveMinColWidth.value, effectiveGap.value)
)

/** Exposed so a virtualizing parent can size its scroll rows from the same math. */
const columnWidth = computed(() =>
    computeColumnWidth(availableWidth.value, columns.value, effectiveGap.value)
)

defineExpose({ columns, columnWidth })

const gridStyle = computed(() => ({
    display: 'grid',
    // minmax(0, ...) — never `minColWidth` — see the note above.
    gridTemplateColumns: `repeat(${columns.value}, minmax(0, 1fr))`,
    gap: `${effectiveGap.value}px`
}))

function measureWidth(): void {
    const w = sizer.value?.clientWidth ?? 0
    if (w > 0) availableWidth.value = w
}

let ro: ResizeObserver | null = null

onMounted(() => {
    measureWidth()
    if (typeof ResizeObserver !== 'undefined' && sizer.value) {
        ro = new ResizeObserver(() => measureWidth())
        ro.observe(sizer.value)
    }
})

onBeforeUnmount(() => ro?.disconnect())
</script>

<template>
    <div ref="sizer" class="card-grid-layout" :style="gridStyle">
        <div v-for="(cell, i) in items" :key="cell?.id ?? `ph-${i}`" class="card-grid-cell">
            <slot name="card" :item="cell" />
        </div>
    </div>
</template>

<style scoped>
.card-grid-layout {
    /* The element is its own width probe, so it must not be sized by its content. */
    width: 100%;
    box-sizing: border-box;
}

/* The other half of the constraint: without min-width:0 a cell adopts its
   content's intrinsic width and the ellipsis never engages. */
.card-grid-cell {
    min-width: 0;
}
</style>
