<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import { useAuth } from '@/composables/useAuth'
import { apiErrorMessage } from '@/lib/apiError'
import { lastServerUrl, sessionLostUnexpectedly } from '@/lib/authState'
import { isDesktopApp, serverUrl, setApiKey, setServerUrl, setSessionKind } from '@/lib/host'
import {
    AetherAnswerError,
    ApiKeyRejectedError,
    InvalidServerAddressError,
    NotAetherServerError,
    ServerUnreachableError,
    probeServer,
    verifyApiKey
} from '@/lib/api/Server'
import BrandMark from '@/components/common/BrandMark.vue'

// Rendered by App.vue in place of the whole app whenever a session is
// required and absent — it is not a route, so there is no /login URL to
// bookmark. A successful login always lands on the home page, regardless
// of the URL the browser sat on while logged out.

const { login, needsServer, needsApiKey, switchServer, serverSaved, apiKeySaved } = useAuth()
const router = useRouter()

// Which card shows. Only the desktop app ever needs the first two: the server
// comes before anything else, and a key session — a portal server, which only a
// key gets into, or the user's own choice — asks for an API key, not a password.
const card = computed<'server' | 'apiKey' | 'login'>(() => {
    if (needsServer.value) return 'server'
    if (needsApiKey.value) return 'apiKey'
    return 'login'
})

function onCardSubmit() {
    if (card.value === 'server') void onConnect()
    else if (card.value === 'apiKey') void onConnectWithKey()
    else void onSubmit()
}

const username = ref('')
const password = ref('')
const rememberMe = ref(false)
const error = ref('')

const canSubmit = computed(
    () => username.value.trim().length > 0 && password.value.length > 0 && !login.isPending.value
)

async function onSubmit() {
    if (!canSubmit.value) return
    error.value = ''
    try {
        await login.mutateAsync({
            username: username.value.trim(),
            password: password.value,
            rememberMe: rememberMe.value
        })
        // A fresh session always starts on the landing page — whatever URL
        // the browser happened to sit on while logged out is not restored.
        await router.replace('/')
    } catch (err: any) {
        // The server answers one uniform 401 for every credential failure.
        if (err?.response?.status === 401) {
            error.value = 'Wrong username or password.'
        } else {
            error.value = apiErrorMessage(err)
        }
        password.value = ''
    }
}

// --- Server step (desktop app only) ---

const desktopApp = isDesktopApp()

const serverAddress = ref('')
const connecting = ref(false)
const serverError = ref('')

// Entering the server step, the field offers the address the user left through
// Change, else the saved server that did not answer — saying so — else nothing.
// What the user types is kept while the step stays up.
watch(
    card,
    (now) => {
        if (now !== 'server') return
        serverAddress.value = lastServerUrl.value ?? serverUrl.value ?? ''
        // A server still set means the step is up because it did not answer.
        serverError.value =
            serverUrl.value === null ? '' : new ServerUnreachableError(serverUrl.value).message
    },
    { immediate: true }
)

const canConnect = computed(() => serverAddress.value.trim().length > 0 && !connecting.value)

async function onConnect() {
    if (!canConnect.value) return
    serverError.value = ''
    connecting.value = true
    try {
        const answer = await probeServer(serverAddress.value)
        // The address as the probe settled it: normalised, and on https when
        // the server redirected there. A portal keeps /api/v0 closed to the
        // app, so only an API key gets in: saved as a key session, the gate
        // shows the key card. Anything else is a cookie session: /me asks the
        // saved server, then the app shows for auth "none", this card's
        // credentials for "native".
        await setServerUrl(answer.url, answer.kind === 'portal' ? 'apiKey' : 'cookie')
        serverSaved()
    } catch (err) {
        serverError.value = connectErrorMessage(err)
    } finally {
        connecting.value = false
    }
}

// Only copy written here, or a message Aether itself wrote, reaches the screen:
// a transport's rejection names the request URL, key included. The probe's and
// the key check's errors carry their own; the player's refusal of an address or
// a key (`{ code, message }` over IPC) explains itself without one.
function connectErrorMessage(err: unknown): string {
    if (
        err instanceof InvalidServerAddressError ||
        err instanceof ServerUnreachableError ||
        err instanceof NotAetherServerError ||
        err instanceof ApiKeyRejectedError ||
        err instanceof AetherAnswerError
    ) {
        return err.message
    }
    if (isHostError(err)) return err.message
    return 'Could not connect to the server.'
}

function isHostError(err: unknown): err is { code: string; message: string } {
    if (typeof err !== 'object' || err === null) return false
    const { code, message } = err as { code?: unknown; message?: unknown }
    return typeof code === 'string' && typeof message === 'string'
}

// `Server · host[:port]` above the credentials: where they are about to go.
const serverHost = computed(() => {
    const url = serverUrl.value
    if (url === null) return ''
    try {
        return new URL(url).host
    } catch {
        // Only a hand-edited settings file gets here; show what it holds.
        return url
    }
})

// --- API key card (desktop app only) ---

const API_KEY_HINT = 'Create one in Aether → User settings → Connected apps.'
const API_KEY_REVOKED = 'Your API key was not accepted — it may have been revoked. Enter a new one.'
const PORTAL_ONLY = 'This server signs you in through its portal, so only an API key works here.'

const apiKeyInput = ref('')
const connectingKey = ref(false)
const keyError = ref('')
// Why the password link led nowhere: the server has no password login to offer.
const keyNotice = ref('')
const switchingMethod = ref(false)

// Entering the key card: a key typed before stays, what was said about it does not.
watch(card, (now) => {
    if (now !== 'apiKey') return
    keyError.value = ''
    keyNotice.value = ''
})

const trimmedKey = computed(() => apiKeyInput.value.trim())
const canConnectWithKey = computed(
    () => trimmedKey.value.length > 0 && !connectingKey.value && !switchingMethod.value
)

// Verify first, seed second, store third: nothing keeps a key the server did not
// accept, and no unverified key reaches the subsonic client. Storing the key
// installs it on that client and enables the identity query, so the identity the
// check answered seeds the gate before — the stored key then renders the app
// from it, without asking getUser a second time.
async function onConnectWithKey() {
    if (!canConnectWithKey.value || serverUrl.value === null) return
    keyError.value = ''
    keyNotice.value = ''
    connectingKey.value = true
    const key = trimmedKey.value
    try {
        const user = await verifyApiKey(serverUrl.value, key)
        apiKeySaved(user)
        await setApiKey(key)
        apiKeyInput.value = ''
        await router.replace('/')
    } catch (err) {
        keyError.value = connectErrorMessage(err)
        // A refused key is not worth correcting by hand; anything else may be
        // transient, so the key stays.
        if (err instanceof ApiKeyRejectedError) apiKeyInput.value = ''
    } finally {
        connectingKey.value = false
    }
}

// From the sign-in card: a key works on every authenticated server.
async function onUseApiKey() {
    error.value = ''
    switchingMethod.value = true
    try {
        await setSessionKind('apiKey')
    } catch (err) {
        error.value = connectErrorMessage(err)
    } finally {
        switchingMethod.value = false
    }
}

// From the key card: re-probe the saved server — a portal server never had a
// /me to remember, and a relaunch fetched none. A password login exists only
// where /me answers native (or none, where nothing is asked at all); a portal,
// or a proxy that lets an anonymous /me through, only takes a key.
async function onUsePassword() {
    if (serverUrl.value === null) return
    keyError.value = ''
    keyNotice.value = ''
    switchingMethod.value = true
    try {
        const answer = await probeServer(serverUrl.value)
        if (answer.kind === 'portal' || answer.me.authMethod === 'proxy-header') {
            keyNotice.value = PORTAL_ONLY
            return
        }
        await setSessionKind('cookie')
        serverSaved()
    } catch (err) {
        keyError.value = connectErrorMessage(err)
    } finally {
        switchingMethod.value = false
    }
}
</script>

<template>
    <div class="login-view">
        <!-- The card is dark in both themes (it mirrors the nav rail), so it
             opts into the dark palette locally: `dark-mode` is PrimeVue's
             darkModeSelector and also carries our own --app-* dark tokens,
             making the inputs, checkbox and button render their dark variants
             even while the app is in light mode. -->
        <form class="login-card dark-mode" @submit.prevent="onCardSubmit">
            <div class="login-brand-row">
                <BrandMark size="2.5rem" />
                <h1 class="login-brand">A<span class="login-brand-accent">e</span>ther</h1>
            </div>

            <!-- Desktop app, no server yet: only the address, nothing to sign
                 in to. -->
            <template v-if="card === 'server'">
                <div class="field">
                    <label for="login-server">Server address</label>
                    <InputText
                        id="login-server"
                        v-model="serverAddress"
                        placeholder="http://aether.local:8075"
                        autocomplete="url"
                        inputmode="url"
                        autocapitalize="off"
                        spellcheck="false"
                        autofocus
                        :disabled="connecting"
                    />
                </div>

                <p v-if="serverError" class="login-error" role="alert">{{ serverError }}</p>

                <Button
                    type="submit"
                    label="Connect"
                    class="login-submit"
                    :loading="connecting"
                    :disabled="!canConnect"
                />
            </template>

            <!-- Desktop app in a key session: a pasted API key instead of a
                 password — the only way into a server behind a portal. -->
            <template v-else-if="card === 'apiKey'">
                <!-- Only for a key the server stopped accepting: after a
                     deliberate logout the user knows why the card is here. -->
                <p v-if="sessionLostUnexpectedly" class="login-expired">{{ API_KEY_REVOKED }}</p>

                <p v-if="serverHost" class="login-server">
                    <span>Server · {{ serverHost }}</span>
                    <button
                        type="button"
                        class="login-link"
                        :disabled="connectingKey || switchingMethod"
                        @click="switchServer()"
                    >
                        Change
                    </button>
                </p>

                <div class="field">
                    <label for="login-api-key">API key</label>
                    <Password
                        v-model="apiKeyInput"
                        input-id="login-api-key"
                        :feedback="false"
                        toggle-mask
                        autocomplete="off"
                        :disabled="connectingKey || switchingMethod"
                        fluid
                    />
                    <p class="login-hint">{{ API_KEY_HINT }}</p>
                </div>

                <p v-if="keyNotice" class="login-notice" role="status">{{ keyNotice }}</p>
                <p v-if="keyError" class="login-error" role="alert">{{ keyError }}</p>

                <Button
                    type="submit"
                    label="Connect"
                    class="login-submit"
                    :loading="connectingKey"
                    :disabled="!canConnectWithKey"
                />

                <button
                    type="button"
                    class="login-link login-switch"
                    data-test="use-password"
                    :disabled="connectingKey || switchingMethod"
                    @click="onUsePassword()"
                >
                    Sign in with a password instead
                </button>
            </template>

            <template v-else>
                <!-- Only for a session lost on its own: after a deliberate logout
                     the user knows why the form is here. -->
                <p v-if="sessionLostUnexpectedly" class="login-expired">
                    Your session has expired — please sign in again.
                </p>

                <p v-if="desktopApp && serverHost" class="login-server">
                    <span>Server · {{ serverHost }}</span>
                    <button
                        type="button"
                        class="login-link"
                        :disabled="login.isPending.value"
                        @click="switchServer()"
                    >
                        Change
                    </button>
                </p>

                <div class="field">
                    <label for="login-username">Username</label>
                    <InputText
                        id="login-username"
                        v-model="username"
                        autocomplete="username"
                        autofocus
                        :disabled="login.isPending.value"
                    />
                </div>

                <div class="field">
                    <label for="login-password">Password</label>
                    <Password
                        v-model="password"
                        input-id="login-password"
                        :feedback="false"
                        toggle-mask
                        autocomplete="current-password"
                        :disabled="login.isPending.value"
                        fluid
                    />
                </div>

                <div class="field field-inline">
                    <Checkbox v-model="rememberMe" input-id="login-remember" binary />
                    <label for="login-remember">Keep me logged in</label>
                </div>

                <p v-if="error" class="login-error" role="alert">{{ error }}</p>

                <Button
                    type="submit"
                    label="Sign in"
                    class="login-submit"
                    :loading="login.isPending.value"
                    :disabled="!canSubmit"
                />

                <!-- Desktop app: a key works on every authenticated server. -->
                <button
                    v-if="desktopApp"
                    type="button"
                    class="login-link login-switch"
                    data-test="use-api-key"
                    :disabled="login.isPending.value || switchingMethod"
                    @click="onUseApiKey()"
                >
                    Use an API key instead
                </button>
            </template>
        </form>
    </div>
</template>

<style scoped>
.login-view {
    display: flex;
    align-items: center;
    justify-content: center;
    /* dvh, not vh: 100vh is the URL-bar-hidden height on mobile browsers, which
       would leave the card off-center and the page scrollable behind them. */
    height: 100dvh;
    /* Moody deep-blue atmosphere (same in both themes — the scene is
       inherently dark, like the nav rail): a wide brand-blue glow rising
       from the lower left and a softer mid-height echo. */
    background-color: #060b13;
    background-image:
        /* main glow, lower left */
        radial-gradient(
            110% 95% at 10% 75%,
            color-mix(in srgb, var(--app-nav-brand) 34%, #0b1c30),
            transparent 72%
        ),
        /* softer mid echo */
        radial-gradient(
            65% 55% at 42% 45%,
            color-mix(in srgb, var(--app-nav-brand) 16%, transparent),
            transparent 70%
        );
    color: var(--app-text-primary);

    /* Snapshot the sidebar palette HERE, outside the card's `dark-mode`
       scope: the card must match the nav rail of the CURRENT app theme
       (#1b2430 light / #0a0e13 dark), but the `dark-mode` class on the card
       would flip --app-nav-* to their dark values before the card could read
       them. Captured on the wrapper, they resolve against the real theme. */
    --login-card-bg: var(--app-nav-bg);
    --login-card-text: var(--app-nav-text);
    --login-card-dim: var(--app-nav-text-dim);
}

.login-card {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    width: min(22rem, calc(100vw - 2rem));
    padding: 2rem;
    border: 1px solid var(--app-border);
    border-radius: 12px;
    background-color: var(--login-card-bg);
    color: var(--login-card-text);

    /* Form fields sit one step lighter than the card — same trick as the
       sidebar's hover tint — instead of PrimeVue's surface-950, which is
       darker than the card and kills the contrast. */
    --c-form-field-background: color-mix(in srgb, var(--login-card-bg), #ffffff 7%);

    /* PrimeVue resolves component tokens (--c-inputtext-*, …) once at :root,
       so the `dark-mode` class on this card only flips the SEMANTIC tokens
       (--c-form-field-*, --c-primary-*). Re-declaring the component tokens
       here makes them re-resolve against those dark semantics, so the form
       controls render their dark variants while the app is in light mode. */
    --c-inputtext-background: var(--c-form-field-background);
    --c-inputtext-border-color: var(--c-form-field-border-color);
    --c-inputtext-hover-border-color: var(--c-form-field-hover-border-color);
    --c-inputtext-focus-border-color: var(--c-form-field-focus-border-color);
    --c-inputtext-color: var(--c-form-field-color);
    --c-inputtext-placeholder-color: var(--c-form-field-placeholder-color);
    --c-checkbox-background: var(--c-form-field-background);
    --c-checkbox-border-color: var(--c-form-field-border-color);
    --c-checkbox-hover-border-color: var(--c-form-field-hover-border-color);
    --c-checkbox-checked-background: var(--c-primary-color);
    --c-checkbox-checked-border-color: var(--c-primary-color);
    --c-checkbox-checked-hover-background: var(--c-primary-hover-color);
    --c-checkbox-checked-hover-border-color: var(--c-primary-hover-color);
    --c-checkbox-icon-checked-color: var(--c-primary-contrast-color);
    --c-checkbox-icon-checked-hover-color: var(--c-primary-contrast-color);
    --c-button-primary-background: var(--c-primary-color);
    --c-button-primary-border-color: var(--c-primary-color);
    --c-button-primary-color: var(--c-primary-contrast-color);
    --c-button-primary-hover-background: var(--c-primary-hover-color);
    --c-button-primary-hover-border-color: var(--c-primary-hover-color);
    --c-button-primary-active-background: var(--c-primary-active-color);
    --c-button-primary-active-border-color: var(--c-primary-active-color);
}

/* Mark and wordmark read as one logo, centered together. */
.login-brand-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.6rem;
    margin-bottom: 0.5rem;
}

.login-brand {
    margin: 0;
    text-align: center;
    font-size: 1.6rem;
    font-weight: 800;
    letter-spacing: 0.02em;
    color: var(--app-nav-brand);
}

.login-brand-accent {
    color: var(--app-nav-brand-alt);
}

.login-expired {
    margin: 0;
    text-align: center;
    font-size: 0.85rem;
    color: var(--login-card-dim);
}

/* The server the credentials go to, muted like the expiry note, with the way
   back to the server step beside it. */
.login-server {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    align-items: baseline;
    gap: 0.25rem 0.6rem;
    margin: 0;
    font-size: 0.85rem;
    color: var(--login-card-dim);
    overflow-wrap: anywhere;
}

/* A text action. --app-accent resolves inside the card's dark-mode scope, so
   it reads on the dark card in both themes. */
.login-link {
    padding: 0;
    border: none;
    background: none;
    font: inherit;
    font-weight: 600;
    color: var(--app-accent);
    cursor: pointer;
}

.login-link:hover:not(:disabled) {
    color: var(--app-accent-hover);
    text-decoration: underline;
}

.login-link:focus-visible {
    outline: 2px solid var(--app-accent);
    outline-offset: 2px;
    border-radius: 2px;
}

.login-link:disabled {
    cursor: default;
    opacity: 0.6;
}

/* The one line under the key field: where a key comes from. */
.login-hint {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.4;
    color: var(--login-card-dim);
}

/* The other way to sign in, below the submit. */
.login-switch {
    align-self: center;
    font-size: 0.85rem;
}

/* Why only a key works on this server. */
.login-notice {
    margin: 0;
    font-size: 0.85rem;
    line-height: 1.5;
}

.field {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
}

.field label {
    font-size: 0.85rem;
    font-weight: 600;
}

.field-inline {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
}

.field-inline label {
    /* A global form style gives labels a bottom margin, which skews the
       flex centering against the checkbox — neutralize it here. */
    margin: 0;
    font-weight: 500;
    cursor: pointer;
}

.login-error {
    margin: 0;
    font-size: 0.85rem;
    color: var(--p-red-400, #f87171);
}

.login-submit {
    margin-top: 0.5rem;
}
</style>
