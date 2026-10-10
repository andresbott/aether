<script lang="ts">
import { ref } from 'vue'

// The server step's address, kept at module level: the gate unmounts this view
// while a new server's /me loads, and whoever comes back through Change must
// find the address they had, to correct it rather than type it again.
const serverAddress = ref('')
</script>

<script setup lang="ts">
// `ref` comes from the module block above: both blocks share one module scope.
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import { useAuth } from '@/composables/useAuth'
import { apiErrorMessage } from '@/lib/apiError'
import { sessionLostUnexpectedly } from '@/lib/authState'
import { isDesktopApp, serverUrl, setServerUrl } from '@/lib/host'
import {
    InvalidServerAddressError,
    NotAetherServerError,
    ServerUnreachableError,
    normalizeServerUrl,
    probeServer
} from '@/lib/api/Server'
import BrandMark from '@/components/common/BrandMark.vue'

// Rendered by App.vue in place of the whole app whenever a session is
// required and absent — it is not a route, so there is no /login URL to
// bookmark. A successful login always lands on the home page, regardless
// of the URL the browser sat on while logged out.

const { login, needsServer, needsPortal, switchServer } = useAuth()
const router = useRouter()

// Which card shows. Only the desktop app ever needs the first two: the server
// comes before anything else, and a portal-only server leaves nothing to sign
// in to.
const card = computed<'server' | 'portal' | 'login'>(() => {
    if (needsServer.value) return 'server'
    if (needsPortal.value) return 'portal'
    return 'login'
})

function onCardSubmit() {
    if (card.value === 'server') void onConnect()
    else if (card.value === 'login') void onSubmit()
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

const PORTAL_MESSAGE =
    'This server signs you in through a web portal, which the desktop app cannot open yet. ' +
    'Open it in a browser, or connect to another server.'

const connecting = ref(false)
const serverError = ref('')
// The address answered as Aether behind a portal: nothing was saved, and the
// field stays open for another address.
const behindPortal = ref(false)

const canConnect = computed(() => serverAddress.value.trim().length > 0 && !connecting.value)

async function onConnect() {
    if (!canConnect.value) return
    serverError.value = ''
    behindPortal.value = false
    connecting.value = true
    try {
        const url = normalizeServerUrl(serverAddress.value)
        const answer = await probeServer(url)
        if (answer.kind === 'portal') {
            behindPortal.value = true
            return
        }
        // Saving is all: the server enables /me, and the gate takes it from
        // there — the app for auth "none", this card's credentials for "native".
        await setServerUrl(url)
    } catch (err) {
        serverError.value = connectErrorMessage(err)
    } finally {
        connecting.value = false
    }
}

// Only copy written here reaches the screen: a transport's rejection names the
// request URL. The probe's errors carry their own; the player's refusal of an
// address (`{ code, message }` over IPC) explains itself without one.
function connectErrorMessage(err: unknown): string {
    if (
        err instanceof InvalidServerAddressError ||
        err instanceof ServerUnreachableError ||
        err instanceof NotAetherServerError
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

// Back to the server step, with the address being left in the field.
function changeServer() {
    if (serverUrl.value !== null) serverAddress.value = serverUrl.value
    void switchServer()
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

                <p v-if="behindPortal" class="login-notice" role="status">
                    {{ PORTAL_MESSAGE }}
                </p>
                <p v-if="serverError" class="login-error" role="alert">{{ serverError }}</p>

                <Button
                    type="submit"
                    label="Connect"
                    class="login-submit"
                    :loading="connecting"
                    :disabled="!canConnect"
                />
            </template>

            <!-- Desktop app on a server that signs in through its portal. -->
            <template v-else-if="card === 'portal'">
                <p class="login-notice">{{ PORTAL_MESSAGE }}</p>
                <Button
                    type="button"
                    label="Change server"
                    class="login-submit"
                    @click="changeServer"
                />
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
                        @click="changeServer"
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

/* Why this server cannot be used from the app. */
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
