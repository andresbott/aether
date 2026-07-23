<script setup lang="ts">
import Button from 'primevue/button'

defineProps<{ title: string; summary?: string; showBack?: boolean }>()
defineEmits<{ (e: 'back'): void }>()
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
       areas (rail slot + the body scroller's scrollbar footprint twice) so the
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

.content-scaffold-body {
    flex: 1;
    min-height: 0;
    /* No gutter here: each body owns its full width so its scrollbar and the
       alphabet rail stay flush right; bodies center content per the recipes in
       docs/architecture/main-content-view-layout.md. */
}
</style>
