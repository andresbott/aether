<script setup lang="ts">
import { computed, ref, useSlots } from 'vue'
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import { useViewport } from '@/composables/useViewport'

defineProps<{ title: string; summary?: string; showBack?: boolean }>()
defineEmits<{ (e: 'back'): void }>()

const slots = useSlots()
const { tier } = useViewport()

// The slot convention (spec §3.2): #actions is always visible; a view that has
// more controls than a phone header fits moves the collapsible ones to
// #secondary-actions. Inline on desktop/tablet, behind ⋮ on phones.
const collapseSecondary = computed(() => tier.value === 'phone' && !!slots['secondary-actions'])

const overflowRef = ref<InstanceType<typeof Popover> | null>(null)
const toggleOverflow = (event: Event) => overflowRef.value?.toggle(event)
</script>

<template>
    <div class="content-scaffold">
        <header class="content-scaffold-header">
            <div class="scaffold-header-inner content-col">
                <Button
                    v-if="showBack"
                    class="scaffold-back"
                    icon="pi pi-arrow-left"
                    text
                    rounded
                    aria-label="Back"
                    @click="$emit('back')"
                />
                <div class="scaffold-title">
                    <h1 v-if="title">{{ title }}</h1>
                    <slot name="title-actions" />
                    <span v-if="summary" class="scaffold-summary">{{ summary }}</span>
                </div>
                <div class="scaffold-actions">
                    <slot name="actions" />
                    <template v-if="!collapseSecondary">
                        <slot name="secondary-actions" />
                    </template>
                    <template v-else>
                        <Button
                            class="scaffold-overflow-btn"
                            icon="pi pi-ellipsis-v"
                            text
                            rounded
                            aria-label="More actions"
                            @click="toggleOverflow"
                        />
                        <Popover ref="overflowRef">
                            <div class="scaffold-overflow-panel">
                                <slot name="secondary-actions" />
                            </div>
                        </Popover>
                    </template>
                </div>
            </div>
        </header>
        <div class="content-scaffold-body">
            <slot />
        </div>
    </div>
</template>

<style scoped>
.content-scaffold {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
}

.content-scaffold-header {
    flex-shrink: 0;
    box-sizing: border-box;
    /* Recipe A: reserve the same right-side clearance as the bodies' scroll
       areas (rail slot + 2×scrollbar: once for the scrollbar the header itself
       doesn't have, once for the clearance the scroller's content adds) so the
       header column sits exactly over the body column. */
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px));
    border-bottom: 1px solid var(--app-border);
}

/* .content-col supplies the centering + inline gutter. */
.scaffold-header-inner {
    display: flex;
    align-items: baseline;
    gap: 1rem;
    padding-top: 0.75rem;
    padding-bottom: 0.75rem;
}

.scaffold-back {
    flex-shrink: 0;
    align-self: center;
}

.scaffold-title {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
}

.scaffold-title h1 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
}

.scaffold-summary {
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--app-text-secondary);
}

.scaffold-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
}

.scaffold-overflow-panel {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

/* Compact header on phones: smaller title, and the title row is allowed to
   wrap so the summary drops below the h1 instead of squeezing it.
   767.98px = $bp-phone-max - 0.02px (guarded by breakpoints.spec.ts). */
@media (max-width: 767.98px) {
    .scaffold-header-inner {
        gap: 0.5rem;
    }

    .scaffold-title {
        flex-wrap: wrap;
        row-gap: 0;
    }

    .scaffold-title h1 {
        font-size: 1.2rem;
    }

    .scaffold-summary {
        flex-basis: 100%;
    }
}

.content-scaffold-body {
    flex: 1;
    min-height: 0;
    /* No gutter here: each body owns its full width so its scrollbar and the
       alphabet rail stay flush right; bodies center content per the recipes in
       docs/architecture/main-content-view-layout.md. */
}
</style>
