<script setup lang="ts">
// Picks how a library sits in the sidebar (`split_views`): one entry whose page
// switches views, or an entry per view. Each choice is a card holding a
// miniature of the app in the current theme — sidebar on the left, page on the
// right, grey bars standing in for text — so the difference reads at a glance.
// Native radios underneath give the group its keyboard and screen-reader
// behaviour; the miniatures are decorative.
const model = defineModel<boolean>({ required: true })

defineProps<{
    // Names the radio group, so two pickers on one page stay independent.
    name: string
    ariaLabelledby?: string
}>()

const options = [
    { value: false, label: 'One entry' },
    { value: true, label: 'Entry per view' }
]
</script>

<template>
    <div class="layout-picker" role="radiogroup" :aria-labelledby="ariaLabelledby">
        <label
            v-for="opt in options"
            :key="opt.label"
            class="layout-card"
            :class="{ selected: model === opt.value }"
        >
            <input
                type="radio"
                class="sr-only"
                :name="name"
                :checked="model === opt.value"
                @change="model = opt.value"
            />
            <div class="mock" aria-hidden="true">
                <div class="mock-sidebar">
                    <template v-if="opt.value">
                        <!-- The library's own section: its header, then a line per view. -->
                        <span class="bar label"></span>
                        <span class="bar active"></span>
                        <span class="bar"></span>
                        <span class="bar"></span>
                    </template>
                    <template v-else>
                        <!-- The Library block, with the library as one line in it. -->
                        <span class="bar label"></span>
                        <span class="bar active"></span>
                    </template>
                </div>
                <div class="mock-page">
                    <div class="mock-header">
                        <span class="bar title"></span>
                        <!-- One entry: the page's header switches the views. -->
                        <span v-if="!opt.value" class="mock-switch">
                            <span class="pill active"></span>
                            <span class="pill"></span>
                            <span class="pill"></span>
                        </span>
                    </div>
                    <div class="mock-grid">
                        <span v-for="n in 6" :key="n" class="tile"></span>
                    </div>
                </div>
            </div>
            <span class="layout-caption">{{ opt.label }}</span>
        </label>
    </div>
</template>

<style scoped>
.layout-picker {
    /* Side by side at any dialog width: the cards shrink rather than wrap. */
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 11rem));
    gap: 0.75rem;
}

.layout-card {
    position: relative; /* anchors the visually hidden radio */
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 0.6rem 0.5rem;
    border: 1px solid var(--app-border);
    border-radius: var(--app-radius);
    background: var(--app-surface);
    cursor: pointer;
    transition: border-color 0.15s, background-color 0.15s;
}

.layout-card:hover {
    border-color: var(--app-text-secondary);
}

.layout-card.selected {
    border-color: var(--app-accent);
    background: var(--app-accent-soft);
}

.layout-card:has(input:focus-visible) {
    outline: 2px solid var(--app-accent);
    outline-offset: 2px;
}

.layout-caption {
    font-size: 0.85rem;
}

.layout-card.selected .layout-caption {
    color: var(--app-accent);
    font-weight: 600;
}

/* The miniature browser: sidebar + page. */
.mock {
    display: flex;
    width: 100%;
    aspect-ratio: 16 / 10;
    border: 1px solid var(--app-border);
    border-radius: 4px;
    overflow: hidden;
    background: var(--app-background);
}

.mock-sidebar {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 30%;
    padding: 8px 0 8px 5px;
    background-color: var(--app-nav-bg);
}

.mock-page {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 7px;
    padding: 8px 7px;
    min-width: 0;
}

.bar {
    display: block;
    height: 4px;
    border-radius: 2px;
    background: color-mix(in srgb, var(--app-nav-text-dim) 55%, transparent);
    width: 75%;
}

.bar.label {
    height: 3px;
    width: 45%;
    margin-bottom: 1px;
    background: color-mix(in srgb, var(--app-nav-text-dim) 35%, transparent);
}

/* The entry of the view shown, like the sidebar's own accent marker. */
.bar.active {
    background: var(--app-accent);
}

.mock-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 4px;
}

.bar.title {
    width: 38%;
    height: 5px;
    background: color-mix(in srgb, var(--app-text-secondary) 55%, transparent);
}

.mock-switch {
    display: flex;
    gap: 2px;
}

.pill {
    display: block;
    width: 9px;
    height: 5px;
    border-radius: 1.5px;
    background: color-mix(in srgb, var(--app-text-secondary) 35%, transparent);
}

.pill.active {
    background: var(--app-accent);
}

.mock-grid {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 4px;
}

.tile {
    border-radius: 2px;
    background: color-mix(in srgb, var(--app-text-secondary) 18%, transparent);
}

.sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
}
</style>
