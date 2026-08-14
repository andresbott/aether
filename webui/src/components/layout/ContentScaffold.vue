<script setup lang="ts">
import { computed, ref, useSlots } from 'vue'
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import { useMobileNav } from '@/composables/useMobileNav'
import { useViewport } from '@/composables/useViewport'

const props = defineProps<{ title: string; summary?: string; showBack?: boolean }>()
defineEmits<{ (e: 'back'): void }>()

const slots = useSlots()
const { tier, shell } = useViewport()

// The mobile shell has no persistent nav chrome (no sidebar, no tab bar), so
// top-level views carry the drawer trigger in their header. Detail views show
// Back in that spot instead — backing out first is the standard drawer UX.
const { isOpen: navOpen, open: openNav } = useMobileNav()
const showNavButton = computed(() => shell.value === 'mobile' && !props.showBack)

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
                    v-if="showNavButton"
                    class="scaffold-nav-btn"
                    icon="pi pi-bars"
                    text
                    rounded
                    aria-label="Open navigation"
                    aria-haspopup="dialog"
                    :aria-expanded="navOpen"
                    @click="openNav"
                />
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
    /* Wraps only on genuine overflow — wide desktop keeps one row; fixes the
       narrow-desktop title crush (phase-3 M9). */
    flex-wrap: wrap;
}

.scaffold-back,
.scaffold-nav-btn {
    flex-shrink: 0;
    align-self: center;
}

.scaffold-title {
    flex: 1;
    min-width: 12rem;
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
}

.scaffold-title:empty {
    min-width: 0;
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
    /* No flex override on .scaffold-title here: the base `flex: 1` +
       `min-width: 12rem` keep the hamburger (or Back) and the title together
       on the first row while a wide #actions (Library's three-option tab
       SelectButton) still wraps below rather than crushing the title. A
       `flex: 1 1 100%` full-row title — the pre-hamburger phone layout —
       would strand the hamburger alone on its own row. When the actions DO
       wrap they land left-aligned; no `margin-left: auto` on
       .scaffold-actions, which would right-align that second row too. */
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
