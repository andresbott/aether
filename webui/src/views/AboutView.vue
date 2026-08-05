<script setup lang="ts">
import { computed } from 'vue'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { useVersion } from '@/composables/useVersion'
import { VISIBLE_SHORTCUTS } from '@/utils/shortcuts'

const { data: serverVersion } = useVersion()

// Same prefix rule as the settings sidebar version string: "v" only before
// bare numeric versions, non-release builds ("dev-build") shown verbatim.
const versionLabel = computed(() => {
    const v = serverVersion.value?.version
    if (!v) return ''
    return /^\d/.test(v) ? `v${v}` : v
})

const commitLabel = computed(() => {
    const c = serverVersion.value?.commit
    if (!c || c === 'undefined') return ''
    return c.slice(0, 8)
})

const buildTime = computed(() => serverVersion.value?.build_time || '')
</script>

<template>
    <ContentScaffold title="About">
        <div class="about-body">
            <div class="content-col">
                <!-- Rendered straight from the shortcut registry, which the in-player
                     help overlay also reads, so the two can never disagree. -->
                <section class="about-section">
                    <h2>Keyboard shortcuts</h2>
                    <p class="setting-hint">
                        Active in the player. Press <kbd>?</kbd> anywhere in the player to see
                        them on the controls themselves.
                    </p>
                    <dl class="shortcuts-table">
                        <div
                            v-for="entry in VISIBLE_SHORTCUTS"
                            :key="entry.action"
                            class="shortcut-row"
                        >
                            <dt class="shortcut-keys">
                                <kbd v-for="key in entry.keys" :key="key">{{ key }}</kbd>
                            </dt>
                            <dd class="shortcut-label">{{ entry.label }}</dd>
                        </div>
                    </dl>
                </section>

                <section class="about-section">
                    <h2>Build</h2>
                    <dl class="info-table">
                        <div class="info-row">
                            <dt>Version</dt>
                            <dd>{{ versionLabel || 'unknown' }}</dd>
                        </div>
                        <div v-if="commitLabel" class="info-row">
                            <dt>Commit</dt>
                            <dd class="mono">{{ commitLabel }}</dd>
                        </div>
                        <div v-if="buildTime" class="info-row">
                            <dt>Built</dt>
                            <dd>{{ buildTime }}</dd>
                        </div>
                    </dl>
                </section>

                <section class="about-section">
                    <h2>Source</h2>
                    <p>
                        Developed by Andres Bott —
                        <a
                            href="https://github.com/andresbott/aether"
                            target="_blank"
                            rel="noopener"
                            >github.com/andresbott/aether</a
                        >
                    </p>
                </section>
            </div>
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* Recipe B (main-content-view-layout.md §4): self-scrolling body on the
   shared content column, uniform rail clearance on the right. */
.about-body {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-top: 1rem;
    padding-bottom: 2rem;
    color: var(--app-text-primary);
}

.about-section {
    margin-top: 2rem;
}

.about-section:first-child {
    margin-top: 0.5rem;
}

.about-section h2 {
    margin: 0 0 0.75rem;
    font-size: 1.1rem;
    font-weight: 600;
}

.about-section p {
    margin: 0;
    color: var(--app-text-secondary);
}

.about-section a {
    color: var(--app-accent);
}

.info-table {
    margin: 0;
    display: grid;
    gap: 0.5rem;
}

.info-row {
    display: flex;
    align-items: baseline;
    gap: 1rem;
}

.info-row dt {
    flex-shrink: 0;
    width: 5.5rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}

.info-row dd {
    margin: 0;
}

.mono {
    font-family: monospace;
}

.setting-hint {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}

.shortcuts-table {
    margin: 1rem 0 0;
    display: grid;
    gap: 0.5rem;
}

.shortcut-row {
    display: flex;
    align-items: baseline;
    gap: 1rem;
}

/* Fixed width so every action label starts on the same column, however many
   keys the binding spells out. */
.shortcut-keys {
    display: flex;
    gap: 0.25rem;
    flex-shrink: 0;
    width: 5.5rem;
    margin: 0;
}

.shortcut-label {
    margin: 0;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}

kbd {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    border: 1px solid var(--app-border);
    background: var(--app-surface);
    color: var(--app-text-primary);
    font-family: inherit;
    font-size: 0.8rem;
    line-height: 1.5;
}
</style>
