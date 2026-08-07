<script setup lang="ts">
import { ref, computed } from 'vue'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Dialog from 'primevue/dialog'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { useTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'
import { useTokens, useCreateToken, useRevokeToken } from '@/composables/useTokens'

const { mode, options, hiddenUnlocked } = useTheme()

// Identity from the /me bootstrap; null with auth method "none", where these
// are device-level settings only.
const { currentUser } = useAuth()

const { data: tokens } = useTokens(computed(() => currentUser.value !== null))
const createToken = useCreateToken()
const revokeToken = useRevokeToken()

const newTokenName = ref('')
const plaintext = ref('')
const plaintextVisible = ref(false)

function onCreate(): void {
    createToken.mutate(
        { name: newTokenName.value.trim() },
        {
            onSuccess: (res) => {
                plaintext.value = res.token
                plaintextVisible.value = true
                newTokenName.value = ''
            }
        }
    )
}

function copyPlaintext(): void {
    void navigator.clipboard?.writeText(plaintext.value)
}

function formatDate(iso: string): string {
    return new Date(iso).toLocaleDateString()
}
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

                <section v-if="currentUser" class="profile-section tokens-section">
                    <h2>API tokens</h2>
                    <p class="setting-hint">
                        Connect third-party Subsonic apps (Symfonium, DSub, …) with a personal
                        access token. Enter it as the app's API key.
                    </p>

                    <div v-if="tokens && tokens.length" class="token-list">
                        <div v-for="tok in tokens" :key="tok.tokenId" class="token-row">
                            <div class="token-meta">
                                <span class="token-name">{{ tok.name }}</span>
                                <span class="setting-hint">
                                    created {{ formatDate(tok.createdAt) }}
                                    <template v-if="tok.lastUsedAt">
                                        · last used {{ formatDate(tok.lastUsedAt) }}
                                    </template>
                                    <template v-if="tok.expiresAt">
                                        · expires {{ formatDate(tok.expiresAt) }}
                                    </template>
                                </span>
                            </div>
                            <Button
                                class="token-revoke"
                                severity="danger"
                                text
                                label="Revoke"
                                :loading="revokeToken.isPending.value"
                                @click="revokeToken.mutate(tok.tokenId)"
                            />
                        </div>
                    </div>
                    <p v-else class="setting-hint">No tokens yet.</p>

                    <form class="token-create" @submit.prevent="onCreate">
                        <InputText
                            v-model="newTokenName"
                            placeholder="Token name (e.g. Symfonium on phone)"
                        />
                        <Button
                            type="submit"
                            label="Create token"
                            :disabled="!newTokenName.trim()"
                            :loading="createToken.isPending.value"
                        />
                    </form>

                    <Dialog
                        v-model:visible="plaintextVisible"
                        modal
                        header="Token created"
                        :closable="true"
                        @hide="plaintext = ''"
                    >
                        <p>Copy it now — it will not be shown again.</p>
                        <code class="token-plaintext">{{ plaintext }}</code>
                        <Button label="Copy" icon="pi pi-copy" @click="copyPlaintext" />
                    </Dialog>
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

.token-list {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-bottom: 1.5rem;
}

.token-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.75rem;
    border: 1px solid var(--app-border);
    border-radius: 0.375rem;
}

.token-meta {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

.token-name {
    font-weight: 500;
}

.token-create {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    flex-wrap: wrap;
}

.token-plaintext {
    display: block;
    padding: 0.75rem;
    margin: 1rem 0;
    background: var(--app-bg-secondary);
    border: 1px solid var(--app-border);
    border-radius: 0.375rem;
    font-family: monospace;
    word-break: break-all;
}
</style>
