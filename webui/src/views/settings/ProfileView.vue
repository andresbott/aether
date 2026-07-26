<script setup lang="ts">
import SelectButton from 'primevue/selectbutton'
import { useTheme } from '@/composables/useTheme'

const { mode, options, hiddenUnlocked } = useTheme()
</script>

<template>
    <div class="profile-view">
        <h1>Profile</h1>
        <p>Profile settings placeholder — user account options will live here once the auth system lands.</p>

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
</style>
