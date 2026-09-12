<script setup lang="ts">
import { computed, ref, useSlots } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import { useViewport } from '@/composables/useViewport'

const props = defineProps<{
    title: string
    summary?: string
    showBack?: boolean
    // Set by the view the hamburger LEADS to (MobileBrowseView), which must not
    // offer a button back to itself.
    navRoot?: boolean
}>()
defineEmits<{ (e: 'back'): void }>()

const slots = useSlots()
const router = useRouter()
const { tier, shell } = useViewport()

// The mobile shell has no persistent nav chrome (no sidebar, no tab bar), so
// top-level views carry the hamburger in their header. It NAVIGATES — /browse is
// the phone's nav surface, a route rather than the overlay drawer it replaced —
// so there is no open state to track and system-back leaves it for free. Detail
// views show Back in that spot instead: backing out before branching off is the
// standard mobile flow.
const showNavButton = computed(
    () => shell.value === 'mobile' && !props.showBack && !props.navRoot
)
const goBrowse = (): void => {
    void router.push({ name: 'browse' })
}

// The slot convention (spec §3.2): #actions is always visible; a view that has
// more controls than a phone header fits moves the collapsible ones to
// #secondary-actions. Inline on desktop/tablet, behind ⋮ on phones.
const collapseSecondary = computed(() => tier.value === 'phone' && !!slots['secondary-actions'])

// A detail view passes no title (its name lives in the hero below), so the header
// is just the Back button + any actions: collapse it to a slimmer bar.
const isNavOnly = computed(() => !props.title && !slots['title-actions'])

const overflowRef = ref<InstanceType<typeof Popover> | null>(null)
const toggleOverflow = (event: Event) => overflowRef.value?.toggle(event)
</script>

<template>
    <div class="content-scaffold">
        <header class="content-scaffold-header" :class="{ 'nav-only': isNavOnly }">
            <div class="scaffold-header-inner content-col">
                <Button
                    v-if="showNavButton"
                    class="scaffold-nav-btn"
                    icon="pi pi-bars"
                    text
                    rounded
                    aria-label="Open navigation"
                    @click="goBrowse"
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

/* Detail views pass no title (the name lives in the hero), so the header is just
   the Back button + actions: slim it down rather than reserving a full title row,
   and center its controls (the base `baseline` alignment would sit an icon-only
   Back button low in the slim bar). */
.content-scaffold-header.nav-only .scaffold-header-inner {
    padding-top: 0.3rem;
    padding-bottom: 0.3rem;
    align-items: center;
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

/* A view's icon-only action button (Playlists/Radio's add) comes in round and
   borderless — it floats beside the square, bordered SelectButton controls
   instead of reading as one of the header controls. Square it off with the same
   height, radius and border as the button groups; the accent colour keeps it
   legible as the action. The phone overflow ⋮ is excluded — it stays a plain
   trigger. Direct children only: a button that is part of a deliberate group
   (a ButtonGroup, nested under .p-buttongroup) owns its own segmented look and
   must not pick up the lone-button accent. */
.scaffold-actions > :deep(.p-button-icon-only:not(.scaffold-overflow-btn)) {
    width: 2.125rem;
    height: 2.125rem;
    padding: 0;
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    border-radius: 6px;
    color: var(--app-accent);
}

.scaffold-actions > :deep(.p-button-icon-only:not(.scaffold-overflow-btn):hover) {
    background: color-mix(in srgb, var(--app-accent) 12%, var(--app-surface-2));
    border-color: var(--app-border);
    color: var(--app-accent);
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
