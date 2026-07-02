<script setup lang="ts">
defineProps<{ title: string; summary?: string }>()
</script>

<template>
    <div class="library-scaffold">
        <header class="library-scaffold-header">
            <div class="scaffold-title">
                <h1>{{ title }}</h1>
                <span v-if="summary" class="scaffold-summary">{{ summary }}</span>
            </div>
            <div class="scaffold-actions">
                <slot name="actions" />
            </div>
        </header>
        <div class="library-scaffold-body">
            <slot />
        </div>
    </div>
</template>

<style scoped>
.library-scaffold {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
}

.library-scaffold-header {
    display: flex;
    align-items: baseline;
    gap: 1rem;
    flex-shrink: 0;
    padding: 0.75rem 1rem;
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

.library-scaffold-body {
    flex: 1;
    min-height: 0;
}
</style>
