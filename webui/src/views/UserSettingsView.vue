<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Dialog from 'primevue/dialog'
import ConfirmDialog from 'primevue/confirmdialog'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import Tag from 'primevue/tag'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { useTheme } from '@/composables/useTheme'
import { useAuth } from '@/composables/useAuth'
import { useTokens, useCreateToken, useRevokeToken } from '@/composables/useTokens'
import { changePassword as apiChangePassword } from '@/lib/api/Auth'
import { apiErrorMessage } from '@/lib/apiError'
import { spaTokenId } from '@/lib/subsonicSession'
import type { ApiToken, ApiTokenType, CreateTokenResponse } from '@/types/tokens'

const { mode, options, hiddenUnlocked } = useTheme()

// Identity from the /me bootstrap; null with auth method "none", where these
// are device-level settings only. authRequired is true only in native mode:
// under proxy-header the identity provider owns the credential, so there is no
// local password to change.
const { currentUser, authRequired, logout } = useAuth()
const toast = useToast()

// The change-password form belongs only to a signed-in native user: proxy-header
// delegates credentials to the IdP (nothing to change here) and auth method
// "none" has no user at all.
const canChangePassword = computed(() => authRequired.value && currentUser.value !== null)

const pwCurrent = ref('')
const pwNew = ref('')
const pwConfirm = ref('')
const pwSubmitting = ref(false)
const pwError = ref('')

async function onChangePassword(): Promise<void> {
    pwError.value = ''
    if (!pwCurrent.value || !pwNew.value) {
        pwError.value = 'Enter your current and new password.'
        return
    }
    if (pwNew.value !== pwConfirm.value) {
        pwError.value = 'The new passwords do not match.'
        return
    }
    pwSubmitting.value = true
    try {
        await apiChangePassword(pwCurrent.value, pwNew.value)
        // A successful change signs this device out: the server clears the
        // session cookie, so end the local session and drop to the login view.
        // The toast rides on the app root and outlives this view's unmount.
        toast.add({
            severity: 'success',
            summary: 'Password changed',
            detail: 'Sign in again with your new password.',
            life: 6000
        })
        logout.mutate()
    } catch (err) {
        pwError.value = apiErrorMessage(err)
    } finally {
        pwSubmitting.value = false
    }
}

const { data: tokens } = useTokens(computed(() => currentUser.value !== null))
// First-party tokens an Aether app minted for itself (this web player today;
// a mobile or desktop app mints the same way).
const sessionTokens = computed(() => tokens.value?.filter((t) => t.kind === 'session') ?? [])
// User-created PATs for third-party Subsonic clients.
const clientTokens = computed(() => tokens.value?.filter((t) => t.kind === 'client') ?? [])
const createToken = useCreateToken()
const revokeToken = useRevokeToken()
const confirm = useConfirm()

interface SettingsTab {
    id: 'general' | 'account' | 'access'
    label: string
    icon: string
}

// The access tab needs a user to own the tokens; with auth method "none" only
// the device-level General settings exist. Account holds the change-password
// form, so it appears only where a password can be changed (native + signed in).
const tabs = computed<SettingsTab[]>(() => [
    { id: 'general', label: 'General', icon: 'pi pi-user' },
    ...(canChangePassword.value
        ? ([{ id: 'account', label: 'Account', icon: 'pi pi-lock' }] as SettingsTab[])
        : []),
    ...(currentUser.value
        ? ([{ id: 'access', label: 'Connected apps', icon: 'pi pi-mobile' }] as SettingsTab[])
        : [])
])

const route = useRoute()
const router = useRouter()

// The default section owns the bare /user-settings path rather than
// /user-settings/general, so the entry point from the user menu stays clean.
const DEFAULT_TAB: SettingsTab['id'] = 'general'

function tabPath(id: SettingsTab['id']): string {
    return id === DEFAULT_TAB ? '/user-settings' : `/user-settings/${id}`
}

// The section is a path segment (/user-settings/access) so a reload, a back
// navigation or a shared link lands on the same panel. An unknown or currently
// unavailable section falls back to the default, so the panel area never goes
// blank.
const activeTab = computed<SettingsTab['id']>({
    get: () => {
        const wanted = route.params.tab
        return tabs.value.some((t) => t.id === wanted) ? (wanted as SettingsTab['id']) : DEFAULT_TAB
    },
    set: (v) => {
        router.replace({ path: tabPath(v), query: route.query })
    }
})

const tablist = ref<HTMLElement | null>(null)

// A URL naming a section that doesn't exist (a typo, or /access after signing
// out) shows the default panel; rewrite the address bar to match what is
// actually on screen instead of leaving a lying URL behind.
watch(
    [tabs, () => route.params.tab],
    ([list, wanted]) => {
        if (wanted && !list.some((t) => t.id === wanted)) {
            router.replace({ path: tabPath(DEFAULT_TAB), query: route.query })
        }
    },
    { immediate: true }
)

/** Vertical tablist keyboard model: arrows wrap, Home/End jump to the ends. */
function onTabKeydown(event: KeyboardEvent, index: number): void {
    const count = tabs.value.length
    let next: number
    switch (event.key) {
        case 'ArrowDown':
        case 'ArrowRight':
            next = (index + 1) % count
            break
        case 'ArrowUp':
        case 'ArrowLeft':
            next = (index - 1 + count) % count
            break
        case 'Home':
            next = 0
            break
        case 'End':
            next = count - 1
            break
        default:
            return
    }
    event.preventDefault()
    activeTab.value = tabs.value[next].id
    tablist.value?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus()
}

function confirmRevoke(tok: ApiToken): void {
    const session = tok.kind === 'session'
    confirm.require({
        message: session
            ? `Revoke this Aether app session ("${tok.name}")? That app is signed out of playback until it reloads.`
            : `Revoke "${tok.name}"? The app or script using it can no longer sign in.`,
        header: session ? 'Revoke session?' : 'Revoke token?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Revoke',
        acceptClass: 'p-button-danger',
        accept: () => revokeToken.mutate(tok.tokenId)
    })
}

const newTokenName = ref('')
const newTokenType = ref<ApiTokenType>('usertoken')
const createVisible = ref(false)
const created = ref<CreateTokenResponse | null>(null)
const createdVisible = ref(false)

const createHeader = computed(() =>
    newTokenType.value === 'usertoken' ? 'Connect a music app' : 'Create an API key'
)

function openCreate(type: ApiTokenType): void {
    newTokenType.value = type
    newTokenName.value = ''
    createVisible.value = true
}

function onCreate(): void {
    const name = newTokenName.value.trim()
    if (!name) return
    createToken.mutate(
        { name, type: newTokenType.value },
        {
            onSuccess: (res) => {
                created.value = res
                createdVisible.value = true
                createVisible.value = false
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
    return new Date(iso).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric'
    })
}

function isCurrentSession(tok: ApiToken): boolean {
    return tok.tokenId === spaTokenId.value
}

const DAY_MS = 24 * 60 * 60 * 1000
const STALE_AFTER_DAYS = 30

/** Humanised last-use with a tone: fresh (≤ yesterday), stale (> 30 days), or neutral. */
function lastUsed(tok: ApiToken): { text: string; tone: 'fresh' | 'stale' | 'none' } {
    if (isCurrentSession(tok)) return { text: 'Active now', tone: 'fresh' }
    if (!tok.lastUsedAt) return { text: 'Never used', tone: 'none' }
    const days = Math.floor((Date.now() - new Date(tok.lastUsedAt).getTime()) / DAY_MS)
    if (days <= 0) return { text: 'Last used today', tone: 'fresh' }
    if (days === 1) return { text: 'Last used yesterday', tone: 'fresh' }
    if (days < 14) return { text: `Last used ${days} days ago`, tone: 'none' }
    const weeks = Math.floor(days / 7)
    if (days <= STALE_AFTER_DAYS) return { text: `Last used ${weeks} weeks ago`, tone: 'none' }
    return { text: `Not used in ${weeks} weeks`, tone: 'stale' }
}
</script>

<template>
    <ContentScaffold title="User settings">
        <div class="profile-body">
            <div class="content-col settings-grid">
                <div class="settings-side">
                    <nav
                        ref="tablist"
                        class="settings-tabs"
                        role="tablist"
                        aria-orientation="vertical"
                        aria-label="User settings sections"
                    >
                        <button
                            v-for="(tab, i) in tabs"
                            :id="`tab-${tab.id}`"
                            :key="tab.id"
                            class="tab-item"
                            :class="{ 'is-active': activeTab === tab.id }"
                            type="button"
                            role="tab"
                            :aria-selected="activeTab === tab.id"
                            :aria-controls="`panel-${tab.id}`"
                            :tabindex="activeTab === tab.id ? 0 : -1"
                            @click="activeTab = tab.id"
                            @keydown="onTabKeydown($event, i)"
                        >
                            <i :class="tab.icon" />
                            <span>{{ tab.label }}</span>
                        </button>
                    </nav>
                </div>

                <!-- Panels stay mounted and are hidden with v-show: they are cheap,
                     and switching tabs keeps scroll position and in-flight form state. -->
                <div class="settings-panels">
                    <section
                        v-show="activeTab === 'general'"
                        id="panel-general"
                        class="settings-panel"
                        role="tabpanel"
                        aria-labelledby="tab-general"
                    >
                        <h2>General</h2>
                        <p class="panel-lead identity-lead">
                            <template v-if="currentUser">
                                Signed in as <strong>{{ currentUser.login }}</strong
                                >.
                            </template>
                            <template v-else>
                                These settings apply to this browser; the server requires no login.
                            </template>
                        </p>

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

                    <section
                        v-if="canChangePassword"
                        v-show="activeTab === 'account'"
                        id="panel-account"
                        class="settings-panel"
                        role="tabpanel"
                        aria-labelledby="tab-account"
                    >
                        <h2>Account</h2>
                        <form class="change-password-form" @submit.prevent="onChangePassword">
                            <h3>Change password</h3>
                            <p class="setting-hint">
                                You'll be signed out and need to sign in again with your new
                                password.
                            </p>
                            <div class="pw-field">
                                <label for="current-password">Current password</label>
                                <InputText
                                    id="current-password"
                                    v-model="pwCurrent"
                                    type="password"
                                    autocomplete="current-password"
                                />
                            </div>
                            <div class="pw-field">
                                <label for="new-password">New password</label>
                                <InputText
                                    id="new-password"
                                    v-model="pwNew"
                                    type="password"
                                    autocomplete="new-password"
                                />
                            </div>
                            <div class="pw-field">
                                <label for="confirm-password">Confirm new password</label>
                                <InputText
                                    id="confirm-password"
                                    v-model="pwConfirm"
                                    type="password"
                                    autocomplete="new-password"
                                />
                            </div>
                            <p v-if="pwError" class="pw-error" role="alert">{{ pwError }}</p>
                            <Button
                                type="submit"
                                label="Change password"
                                :loading="pwSubmitting"
                            />
                        </form>
                    </section>

                    <section
                        v-if="currentUser"
                        v-show="activeTab === 'access'"
                        id="panel-access"
                        class="settings-panel tokens-section"
                        role="tabpanel"
                        aria-labelledby="tab-access"
                    >
                        <h2>Connected apps</h2>
                        <p class="panel-lead">
                            Third-party Subsonic apps and scripts that can sign in as you, plus the
                            Aether apps you are signed in to.
                        </p>

                        <div class="intent-cards">
                            <button
                                type="button"
                                class="intent-card"
                                @click="openCreate('usertoken')"
                            >
                                <span class="intent-ico"><i class="pi pi-mobile" /></span>
                                <span class="intent-title">Connect a music app</span>
                                <span class="setting-hint">
                                    Generate a username + password for a Subsonic app's login form —
                                    Symfonium, DSub, play:Sub, …
                                </span>
                                <span class="intent-cta">Create app password →</span>
                            </button>
                            <button type="button" class="intent-card" @click="openCreate('apikey')">
                                <span class="intent-ico"><i class="pi pi-key" /></span>
                                <span class="intent-title">Create an API key</span>
                                <span class="setting-hint">
                                    A single key for OpenSubsonic clients and your own scripts that
                                    support API-key auth.
                                </span>
                                <span class="intent-cta">Create API key →</span>
                            </button>
                        </div>

                        <div class="token-group">
                            <h3>Apps &amp; API keys</h3>
                            <p class="setting-hint">
                                Third-party Subsonic apps and scripts that can sign in as you.
                            </p>
                            <div v-if="clientTokens.length" class="token-list">
                                <div
                                    v-for="tok in clientTokens"
                                    :key="tok.tokenId"
                                    class="token-row"
                                >
                                    <span class="token-ico">
                                        <i
                                            :class="
                                                tok.type === 'usertoken'
                                                    ? 'pi pi-mobile'
                                                    : 'pi pi-key'
                                            "
                                        />
                                    </span>
                                    <div class="token-meta">
                                        <span class="token-name">
                                            {{ tok.name }}
                                            <Tag
                                                :value="typeLabel(tok.type)"
                                                :severity="
                                                    tok.type === 'usertoken'
                                                        ? undefined
                                                        : 'secondary'
                                                "
                                            />
                                        </span>
                                        <span class="token-when">
                                            <span :class="`tone-${lastUsed(tok).tone}`">{{
                                                lastUsed(tok).text
                                            }}</span>
                                            · added {{ formatDate(tok.createdAt) }}
                                            <template v-if="tok.expiresAt">
                                                · expires {{ formatDate(tok.expiresAt) }}
                                            </template>
                                        </span>
                                    </div>
                                    <Button
                                        v-tooltip.left="'Revoke'"
                                        class="token-revoke"
                                        severity="danger"
                                        text
                                        rounded
                                        icon="pi pi-trash"
                                        :aria-label="`Revoke ${tok.name}`"
                                        :loading="
                                            revokeToken.isPending.value &&
                                            revokeToken.variables.value === tok.tokenId
                                        "
                                        @click="confirmRevoke(tok)"
                                    />
                                </div>
                            </div>
                            <p v-else class="setting-hint">No connected apps yet.</p>
                        </div>

                        <div class="token-group">
                            <h3>Your Aether apps</h3>
                            <p class="setting-hint">
                                One session per Aether app you are signed in to. Revoking one signs
                                that app out until it reloads.
                            </p>
                            <div v-if="sessionTokens.length" class="token-list">
                                <div
                                    v-for="tok in sessionTokens"
                                    :key="tok.tokenId"
                                    class="token-row"
                                >
                                    <span class="token-ico is-neutral"
                                        ><i class="pi pi-desktop"
                                    /></span>
                                    <div class="token-meta">
                                        <span class="token-name">
                                            {{ tok.name }}
                                            <Tag
                                                v-if="isCurrentSession(tok)"
                                                value="This app"
                                                severity="secondary"
                                            />
                                        </span>
                                        <span class="token-when">
                                            <span :class="`tone-${lastUsed(tok).tone}`">{{
                                                lastUsed(tok).text
                                            }}</span>
                                            · started {{ formatDate(tok.createdAt) }}
                                            <template v-if="tok.expiresAt">
                                                · expires {{ formatDate(tok.expiresAt) }}
                                            </template>
                                        </span>
                                    </div>
                                    <Button
                                        v-tooltip.left="
                                            isCurrentSession(tok)
                                                ? 'Sign this browser out'
                                                : 'Revoke session'
                                        "
                                        class="token-revoke"
                                        severity="danger"
                                        text
                                        rounded
                                        icon="pi pi-trash"
                                        :aria-label="`Revoke session ${tok.name}`"
                                        :loading="
                                            revokeToken.isPending.value &&
                                            revokeToken.variables.value === tok.tokenId
                                        "
                                        @click="confirmRevoke(tok)"
                                    />
                                </div>
                            </div>
                            <p v-else class="setting-hint">No active sessions.</p>
                        </div>
                    </section>
                </div>
            </div>
        </div>

        <!-- Overlays live outside the panels: PrimeVue teleports them to the body,
             so they must not depend on which tab is currently shown. -->
        <template v-if="currentUser">
            <ConfirmDialog />

            <Dialog v-model:visible="createVisible" modal :header="createHeader" :closable="true">
                <p v-if="newTokenType === 'usertoken'" class="dialog-lead">
                    Name the connection, then enter the generated username and password in the app's
                    login form.
                </p>
                <p v-else class="dialog-lead">
                    Name the key so you can recognise and revoke it later.
                </p>
                <form id="token-create-form" class="token-create" @submit.prevent="onCreate">
                    <label class="token-create-label" for="token-name">Name</label>
                    <InputText
                        id="token-name"
                        v-model="newTokenName"
                        autofocus
                        :placeholder="
                            newTokenType === 'usertoken'
                                ? 'e.g. Symfonium on phone'
                                : 'e.g. beets import script'
                        "
                    />
                </form>
                <template #footer>
                    <Button label="Cancel" text @click="createVisible = false" />
                    <Button
                        :label="
                            newTokenType === 'usertoken' ? 'Create app password' : 'Create API key'
                        "
                        :disabled="!newTokenName.trim()"
                        :loading="createToken.isPending.value"
                        @click="onCreate"
                    />
                </template>
            </Dialog>

            <Dialog
                v-model:visible="createdVisible"
                modal
                :header="created?.type === 'usertoken' ? 'App password created' : 'API key created'"
                :closable="true"
                @hide="created = null"
            >
                <template v-if="created?.type === 'usertoken'">
                    <p>
                        Enter these in the app's login form, with this server's address as the
                        server. Copy both now — the password will not be shown again.
                    </p>
                    <div class="credential-row">
                        <span class="credential-label">Username</span>
                        <code class="token-plaintext">{{ created.username }}</code>
                        <Button
                            label="Copy"
                            icon="pi pi-copy"
                            text
                            @click="copyText(created?.username)"
                        />
                    </div>
                    <div class="credential-row">
                        <span class="credential-label">Password</span>
                        <code class="token-plaintext">{{ created.password }}</code>
                        <Button
                            label="Copy"
                            icon="pi pi-copy"
                            text
                            @click="copyText(created?.password)"
                        />
                    </div>
                </template>
                <template v-else>
                    <p>
                        Enter this as the API key in your OpenSubsonic client. Copy it now — it will
                        not be shown again.
                    </p>
                    <div class="credential-row">
                        <span class="credential-label">API key</span>
                        <code class="token-plaintext">{{ created?.token }}</code>
                        <Button
                            label="Copy"
                            icon="pi pi-copy"
                            text
                            @click="copyText(created?.token)"
                        />
                    </div>
                </template>
            </Dialog>
        </template>
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

/* --- Vertical tab frame ----------------------------------------------------- */
.settings-grid {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    gap: 2.5rem;
    align-items: start;
}

.settings-side {
    display: flex;
    flex-direction: column;
    /* The rail stays put while a long panel scrolls past it. */
    position: sticky;
    top: 0;
}

/* Square, flush rows — the same rail idiom as the admin SettingsLayout nav. */
.settings-tabs {
    display: flex;
    flex-direction: column;
}

.tab-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: none;
    border-radius: 0;
    background: none;
    font: inherit;
    font-size: 0.9rem;
    color: var(--app-text-secondary);
    text-align: left;
    cursor: pointer;
    transition:
        background-color 0.15s,
        color 0.15s;
}

.tab-item i {
    font-size: 1rem;
    flex-shrink: 0;
}

.tab-item:hover {
    background: var(--app-surface-2);
    color: var(--app-text-primary);
}

.tab-item.is-active {
    background: var(--app-accent-soft);
    color: var(--app-accent);
    font-weight: 600;
    box-shadow: inset 3px 0 0 var(--app-accent);
}

.tab-item:focus-visible {
    outline: 2px solid var(--app-accent);
    outline-offset: 2px;
}

/* --- Panels ----------------------------------------------------------------- */
.settings-panel {
    max-width: 60rem;
}

.settings-panel h2 {
    margin: 0 0 0.35rem;
    font-size: 1.1rem;
    font-weight: 600;
}

/* Scoped under .settings-panel to outweigh the `.profile-body p { margin: 0 }`
   reset above, which is the more specific selector on its own. */
.settings-panel .panel-lead {
    margin: 0 0 1.5rem;
    font-size: 0.9rem;
}

/* The identity line states a fact rather than describing the controls under it,
   so it gets a slightly wider separation than a normal panel lead. */
.settings-panel .identity-lead {
    margin-bottom: 1.85rem;
}

@media (max-width: 860px) {
    .settings-grid {
        grid-template-columns: minmax(0, 1fr);
        gap: 1.5rem;
    }

    .settings-side {
        position: static;
    }

    /* Stacked: the rail turns into a scrollable strip of tabs above the panel. */
    .settings-tabs {
        flex-direction: row;
        overflow-x: auto;
        scrollbar-width: none;
        border-bottom: 1px solid var(--app-border);
    }

    .settings-tabs::-webkit-scrollbar {
        display: none;
    }

    .tab-item {
        width: auto;
        white-space: nowrap;
    }

    .tab-item.is-active {
        box-shadow: inset 0 -3px 0 var(--app-accent);
    }
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

/* --- Change password ------------------------------------------------------- */
.change-password-form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-top: 2rem;
    max-width: 22rem;
}

.change-password-form h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
}

.change-password-form .setting-hint {
    margin: 0;
}

.pw-field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

.pw-field label {
    font-size: 0.85rem;
    font-weight: 500;
}

.pw-field :deep(.p-inputtext) {
    width: 100%;
}

.change-password-form .p-button {
    align-self: flex-start;
}

.pw-error {
    margin: 0;
    color: var(--app-danger, #d32f2f);
    font-size: 0.85rem;
}

/* --- Intent cards: the two create entry points ----------------------------- */
.intent-cards {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
}

@media (max-width: 640px) {
    .intent-cards {
        grid-template-columns: 1fr;
    }
}

.intent-card {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.3rem;
    text-align: left;
    font: inherit;
    color: inherit;
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    border-radius: 0;
    padding: 1.2rem 1.25rem 1.1rem;
    cursor: pointer;
    transition:
        border-color 0.15s,
        box-shadow 0.15s,
        transform 0.15s;
}

.intent-card:hover {
    border-color: var(--app-accent);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.1);
    transform: translateY(-2px);
}

.intent-ico {
    width: 42px;
    height: 42px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 0;
    background: var(--app-accent-soft);
    color: var(--app-accent);
    font-size: 1.2rem;
    margin-bottom: 0.4rem;
}

.intent-title {
    font-size: 1rem;
    font-weight: 600;
}

.intent-cta {
    font-size: 0.83rem;
    font-weight: 700;
    color: var(--app-accent);
    margin-top: 0.45rem;
}

/* --- Token groups: icon rows ------------------------------------------------ */
.token-group {
    margin-top: 2rem;
}

.token-group h3 {
    margin: 0 0 0.15rem;
    font-size: 0.95rem;
    font-weight: 600;
}

.token-group > .setting-hint {
    display: block;
    margin-bottom: 0.75rem;
}

.token-list {
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    border-radius: 0;
    overflow: hidden;
}

.token-row {
    display: flex;
    align-items: center;
    gap: 0.9rem;
    padding: 0.75rem 1.1rem;
}

.token-row + .token-row {
    border-top: 1px solid var(--app-border);
}

.token-ico {
    width: 38px;
    height: 38px;
    flex-shrink: 0;
    border-radius: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: var(--app-accent-soft);
    color: var(--app-accent);
    font-size: 1.05rem;
}

.token-ico.is-neutral {
    background: var(--app-bg-subtle);
    color: var(--app-text-secondary);
}

.token-meta {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
    flex: 1;
}

.token-name {
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
}

.token-when {
    font-size: 0.82rem;
    color: var(--app-text-secondary);
}

.token-when .tone-fresh {
    color: var(--p-green-600, #16a34a);
    font-weight: 600;
}

.token-when .tone-stale {
    color: var(--p-red-500, #ef4444);
    font-weight: 600;
}

/* --- Create dialog ------------------------------------------------------------ */
.dialog-lead {
    margin: 0 0 0.9rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
    max-width: 32rem;
}

.token-create {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
}

.token-create-label {
    font-size: 0.85rem;
    font-weight: 600;
}

.token-create .p-inputtext {
    width: 24rem;
    max-width: 100%;
}

.token-plaintext {
    display: block;
    padding: 0.75rem;
    margin: 1rem 0;
    background: var(--app-bg-subtle);
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
