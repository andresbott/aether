<script setup lang="ts">
import { ref } from 'vue'

// One collapsible editor section (Song, Artists, Album, Attached pictures…).
// Expanded by default; the chevron/title toggles. The body uses v-show so
// collapsed inputs keep their edit buffers and staged state.
const props = defineProps<{
    title: string
    // Optional help tooltip shown next to the title.
    help?: string
    // Colors the title with the staged accent when the section holds edits.
    dirty?: boolean
}>()

const collapsed = ref(false)
</script>

<template>
    <div class="edit-section" :class="{ 'section-dirty': props.dirty }">
        <div class="edit-section-header">
            <button
                type="button"
                class="collapse-toggle"
                :aria-expanded="!collapsed"
                :aria-label="`${collapsed ? 'Expand' : 'Collapse'} ${title}`"
                data-test="section-toggle"
                @click="collapsed = !collapsed"
            >
                <i
                    class="pi collapse-chevron"
                    :class="collapsed ? 'pi-chevron-right' : 'pi-chevron-down'"
                ></i>
                <h4>
                    {{ title }}
                    <i
                        v-if="help"
                        class="pi pi-question-circle field-help"
                        v-tooltip.right="help"
                        data-test="section-help"
                        @click.stop
                    ></i>
                </h4>
            </button>
            <div class="header-actions">
                <slot name="actions" />
            </div>
        </div>
        <div v-show="!collapsed" class="section-body" data-test="section-body">
            <slot />
        </div>
    </div>
</template>

<style scoped>
.edit-section {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-top: 0.5rem;
}
.edit-section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 2rem;
    padding-bottom: 0.25rem;
    border-bottom: 1px solid var(--app-border);
}
.collapse-toggle {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: inherit;
    text-align: left;
}
.collapse-chevron {
    font-size: 0.7rem;
    color: var(--app-text-secondary);
}
.edit-section-header h4 {
    margin: 0;
    font-size: 0.9rem;
}
.section-dirty .edit-section-header h4 {
    color: var(--app-staged);
}
.header-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
}
.field-help {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    cursor: help;
    vertical-align: middle;
    margin-left: 0.15rem;
}
.section-body {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}
</style>
