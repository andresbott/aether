<script setup lang="ts">
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { useTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'

const { mode, options, hiddenUnlocked } = useTheme()

// Identity from the /me bootstrap; null with auth method "none", where these
// are device-level settings only.
const { currentUser } = useAuth()
</script>

<template>
    <ContentScaffold title="User settings">
        <div class="profile-body">
            <div class="content-col">
                <p v-if="currentUser">
                    Signed in as <strong>{{ currentUser.login }}</strong
                    >.
                </p>
                <p v-else>These settings apply to this browser; the server requires no login.</p>

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
        </div>
    </ContentScaffold>
</template>

<style scoped>
/* Recipe B (main-content-view-layout.md §4): the body scrolls itself and
   reserves the uniform rail clearance so the content column sits at the same
   x as every other main view. */
.profile-body {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    padding-top: 1rem;
    padding-bottom: 2rem;
    color: var(--app-text-primary);
}

.profile-body p {
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
