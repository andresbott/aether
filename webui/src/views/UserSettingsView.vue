<script setup lang="ts">
import { ref, computed } from 'vue'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Dialog from 'primevue/dialog'
import Tag from 'primevue/tag'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { useTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'
import { useTokens, useCreateToken, useRevokeToken } from '@/composables/useTokens'
import type { ApiTokenType, CreateTokenResponse } from '@/types/tokens'

const { mode, options, hiddenUnlocked } = useTheme()

// Identity from the /me bootstrap; null with auth method "none", where these
// are device-level settings only.
const { currentUser } = useAuth()

const { data: tokens } = useTokens(computed(() => currentUser.value !== null))
// First-party tokens minted by the Aether web player itself.
const sessionTokens = computed(() => tokens.value?.filter((t) => t.kind === 'session') ?? [])
// User-created PATs for third-party Subsonic clients.
const clientTokens = computed(() => tokens.value?.filter((t) => t.kind === 'client') ?? [])
const createToken = useCreateToken()
const revokeToken = useRevokeToken()

const newTokenName = ref('')
const newTokenType = ref<ApiTokenType>('usertoken')
const tokenTypeOptions = [
    { label: 'App password', value: 'usertoken' },
    { label: 'API key', value: 'apikey' }
]
const created = ref<CreateTokenResponse | null>(null)
const createdVisible = ref(false)

function onCreate(): void {
    createToken.mutate(
        { name: newTokenName.value.trim(), type: newTokenType.value },
        {
            onSuccess: (res) => {
                created.value = res
                createdVisible.value = true
                newTokenName.value = ''
            }
        }
    )
}

function copyText(text: string | undefined): void {
    if (text) void navigator.clipboard?.writeText(text)
}

function typeLabel(type: ApiTokenType): string {
    return type === 'usertoken' ? 'App password' : 'API key'
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

                    <div class="token-group">
                        <h3>Aether sessions</h3>
                        <p class="setting-hint">
                            Tokens the Aether web player mints for its own playback. Revoking
                            one signs that browser's player out until it reloads.
                        </p>
                        <p v-if="tokens" class="token-count">
                            {{ sessionTokens.length }} active Aether
                            {{ sessionTokens.length === 1 ? 'session' : 'sessions' }}
                        </p>
                        <div v-if="sessionTokens.length" class="token-list">
                            <div v-for="tok in sessionTokens" :key="tok.tokenId" class="token-row">
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
                                    :loading="revokeToken.isPending.value && revokeToken.variables.value === tok.tokenId"
                                    @click="revokeToken.mutate(tok.tokenId)"
                                />
                            </div>
                        </div>
                    </div>

                    <div class="token-group">
                        <h3>Third-party tokens</h3>
                        <p class="setting-hint">
                            Connect third-party Subsonic apps with an app password (username +
                            password), or create an API key for OpenSubsonic clients and scripts.
                        </p>
                        <div v-if="clientTokens.length" class="token-list">
                            <div v-for="tok in clientTokens" :key="tok.tokenId" class="token-row">
                                <div class="token-meta">
                                    <span class="token-name">{{ tok.name }}</span>
                                    <Tag :value="typeLabel(tok.type)" severity="secondary" />
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
                                    :loading="revokeToken.isPending.value && revokeToken.variables.value === tok.tokenId"
                                    @click="revokeToken.mutate(tok.tokenId)"
                                />
                            </div>
                        </div>
                        <p v-else class="setting-hint">No tokens yet.</p>
                        <form class="token-create" @submit.prevent="onCreate">
                            <label>Type</label>
                            <SelectButton
                                v-model="newTokenType"
                                :options="tokenTypeOptions"
                                optionLabel="label"
                                optionValue="value"
                                :allowEmpty="false"
                                aria-label="Token type"
                            />
                            <p v-if="newTokenType === 'usertoken'" class="setting-hint">
                                For Subsonic apps (Symfonium, DSub, …): enter the generated
                                username and password in the app's login form.
                            </p>
                            <p v-else class="setting-hint">
                                For clients and scripts that support OpenSubsonic API key
                                authentication.
                            </p>
                            <label for="token-name">Token name</label>
                            <div class="token-create-row">
                                <InputText
                                    id="token-name"
                                    v-model="newTokenName"
                                    placeholder="e.g. Symfonium on phone"
                                />
                                <Button
                                    type="submit"
                                    label="Create token"
                                    :disabled="!newTokenName.trim()"
                                    :loading="createToken.isPending.value"
                                />
                            </div>
                        </form>
                    </div>

                    <Dialog
                        v-model:visible="createdVisible"
                        modal
                        :header="created?.type === 'usertoken' ? 'App password created' : 'Token created'"
                        :closable="true"
                        @hide="created = null"
                    >
                        <template v-if="created?.type === 'usertoken'">
                            <p>
                                Enter these in the app's login form, with this server's address as
                                the server. Copy both now — the password will not be shown again.
                            </p>
                            <div class="credential-row">
                                <span class="credential-label">Username</span>
                                <code class="token-plaintext">{{ created.username }}</code>
                                <Button label="Copy" icon="pi pi-copy" text @click="copyText(created?.username)" />
                            </div>
                            <div class="credential-row">
                                <span class="credential-label">Password</span>
                                <code class="token-plaintext">{{ created.password }}</code>
                                <Button label="Copy" icon="pi pi-copy" text @click="copyText(created?.password)" />
                            </div>
                        </template>
                        <template v-else>
                            <p>Copy it now — it will not be shown again.</p>
                            <code class="token-plaintext">{{ created?.token }}</code>
                            <Button label="Copy" icon="pi pi-copy" @click="copyText(created?.token)" />
                        </template>
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

.token-group {
    margin-top: 1.25rem;
    padding: 1rem;
    border: 1px solid var(--app-border);
    border-radius: 0.5rem;
}

.token-group h3 {
    margin: 0 0 0.25rem;
    font-size: 0.95rem;
    font-weight: 600;
}

.token-group > .setting-hint {
    display: block;
    margin-bottom: 1rem;
}

/* p.token-count so it out-specifies the .profile-body p reset above */
p.token-count {
    margin: 0 0 0.75rem;
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text-primary);
}

.token-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
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
    flex-direction: column;
    gap: 0.35rem;
    margin-top: 1.25rem;
}

.token-create label {
    font-size: 0.85rem;
    font-weight: 600;
}

.token-create-row {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    flex-wrap: wrap;
}

.token-create-row .p-inputtext {
    width: 20rem;
    max-width: 100%;
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

.credential-row {
    display: grid;
    grid-template-columns: 6rem 1fr auto;
    align-items: center;
    gap: 0.75rem;
    margin: 0.5rem 0;
}

.credential-label {
    font-size: 0.85rem;
    font-weight: 600;
}

.credential-row .token-plaintext {
    margin: 0;
}
</style>
