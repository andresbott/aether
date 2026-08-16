<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import { useAuth } from '@/composables/useAuth'
import { apiErrorMessage } from '@/lib/apiError'
import { sessionLostUnexpectedly } from '@/lib/authState'
import BrandMark from '@/components/common/BrandMark.vue'

// Rendered by App.vue in place of the whole app whenever a session is
// required and absent — it is not a route, so there is no /login URL to
// bookmark. A successful login always lands on the home page, regardless
// of the URL the browser sat on while logged out.

const { login } = useAuth()
const router = useRouter()

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
</script>

<template>
    <div class="login-view">
        <!-- The card is dark in both themes (it mirrors the nav rail), so it
             opts into the dark palette locally: `dark-mode` is PrimeVue's
             darkModeSelector and also carries our own --app-* dark tokens,
             making the inputs, checkbox and button render their dark variants
             even while the app is in light mode. -->
        <form class="login-card dark-mode" @submit.prevent="onSubmit">
            <div class="login-brand-row">
                <BrandMark size="2.5rem" />
                <h1 class="login-brand">A<span class="login-brand-accent">e</span>ther</h1>
            </div>
            <!-- Only for a session lost on its own: after a deliberate logout
                 the user knows why the form is here. -->
            <p v-if="sessionLostUnexpectedly" class="login-expired">
                Your session has expired — please sign in again.
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
