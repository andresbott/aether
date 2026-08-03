<script setup lang="ts">
import SelectButton from 'primevue/selectbutton'
import { useTheme } from '@/composables/useTheme'
import { VISIBLE_SHORTCUTS } from '@/utils/shortcuts'

const { mode, options, hiddenUnlocked } = useTheme()
</script>

<template>
    <div class="profile-view">
        <h1>Profile</h1>
        <p>
            Profile settings placeholder — user account options will live here once the auth system
            lands.
        </p>

        <section class="profile-section">
            <h2>Appearance</h2>
            <div class="setting-row">
                <div class="setting-label">
                    <span class="setting-title">Theme</span>
                    <span class="setting-hint">
                        Auto follows your system light/dark preference.
                        <template v-if="hiddenUnlocked"> Nice find.</template>
                    </span>
                </div>
                <SelectButton
                    v-model="mode"
                    :options="options"
                    optionLabel="label"
                    optionValue="value"
                    :allowEmpty="false"
                    aria-label="Theme"
                />
            </div>
        </section>

        <!-- Rendered straight from the shortcut registry, which the in-player
             help overlay also reads, so the two can never disagree. -->
        <section class="profile-section">
            <h2>Keyboard shortcuts</h2>
            <p class="setting-hint">
                Active in the player. Press <kbd>?</kbd> anywhere in the player to see them on the
                controls themselves.
            </p>
            <dl class="shortcuts-table">
                <div v-for="entry in VISIBLE_SHORTCUTS" :key="entry.action" class="shortcut-row">
                    <dt class="shortcut-keys">
                        <kbd v-for="key in entry.keys" :key="key">{{ key }}</kbd>
                    </dt>
                    <dd class="shortcut-label">{{ entry.label }}</dd>
                </div>
            </dl>
        </section>
    </div>
</template>

<style scoped>
.profile-view {
    color: var(--app-text-primary);
    padding: 1.5rem 2rem;
}

.profile-view h1 {
    margin: 0 0 0.5rem;
    font-size: 1.5rem;
}

.profile-view > p {
    color: var(--app-text-secondary);
    margin: 0;
}

.profile-section {
    margin-top: 2rem;
}

.profile-section h2 {
    margin: 0 0 1rem;
    font-size: 1.1rem;
    font-weight: 600;
}

.setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1.5rem;
    flex-wrap: wrap;
}

.setting-label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

.setting-title {
    font-weight: 500;
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
