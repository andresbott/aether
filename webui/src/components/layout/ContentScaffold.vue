<script setup lang="ts">
defineProps<{ title: string; summary?: string }>()
</script>

<template>
    <div class="content-scaffold">
        <header class="content-scaffold-header">
            <div class="scaffold-title">
                <h1>{{ title }}</h1>
                <slot name="title-actions" />
                <span v-if="summary" class="scaffold-summary">{{ summary }}</span>
            </div>
            <div class="scaffold-actions">
                <slot name="actions" />
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
    display: flex;
    align-items: baseline;
    gap: 1rem;
    flex-shrink: 0;
    /* Center on the same content column as the list/grid content instead of
       spanning the full width, so the title sits above the content column. The
       2rem padding is the side gutter and gives the title a small indent. */
    max-width: var(--app-content-max-width);
    width: 100%;
    margin-inline: auto;
    box-sizing: border-box;
    padding: 0.75rem 2rem;
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
    /* Match the header's left gutter so rows line up under the title, but leave
       the right edge unpadded so the list's scroll bar sits flush right. */
    padding-left: 2rem;
}
</style>
