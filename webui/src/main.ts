import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin, QueryClient, QueryCache, MutationCache } from '@tanstack/vue-query'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import Tooltip from 'primevue/tooltip'
import FocusTrap from 'primevue/focustrap'
import StyleClass from 'primevue/styleclass'
import Ripple from 'primevue/ripple'

import CustomTheme from './theme'
import { useTheme } from './composables/useTheme'
import { apiErrorMessage, NETWORK_MESSAGE } from './lib/apiError'
import {
    reportNetworkError,
    dismissBanner,
    onReconnect,
    setConnectivityProbe
} from './composables/useConnectivity'
import { initHost } from './lib/host'
import { isNetworkFailure } from './lib/retry'
import { subsonicClient } from './lib/api/subsonic'
import { getVersion } from './lib/api/Version'
import 'primeflex/primeflex.css'
import 'virtual:material-symbols.css'
import '@fontsource-variable/inter'
import './assets/scss/_main.scss'

import App from './App.vue'
import router from './router'

const app = createApp(App)

// Follow the OS light/dark preference by default; toggles the `.dark-mode`
// class PrimeVue watches (see darkModeSelector below).
useTheme()

app.directive('focus', {
    mounted(el: HTMLElement) {
        el.focus()
        setTimeout(() => {
            el.focus()
        }, 300)
    }
})

app.use(PrimeVue, {
    theme: {
        preset: CustomTheme,
        options: {
            prefix: 'c',
            darkModeSelector: '.dark-mode',
            cssLayer: false
        }
    },
    locale: {
        firstDayOfWeek: 1
    },
    ripple: true
})

app.directive('tooltip', Tooltip)
app.directive('styleclass', StyleClass)
app.directive('focustrap', FocusTrap)
app.directive('ripple', Ripple)
app.use(ToastService)
app.use(ConfirmationService)

app.use(createPinia())
app.use(router)

// A connectivity failure, whichever client it came through: axios' "no response"
// (apiErrorMessage's NETWORK_MESSAGE) or a /rest fetch that never got an answer.
const isConnectivityError = (error: unknown): boolean =>
    isNetworkFailure(error) || apiErrorMessage(error) === NETWORK_MESSAGE

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            refetchOnWindowFocus: false,
            // 'always', not the default 'online': vue-query would otherwise park
            // every query in a "paused" state on the browser's 'offline' event and
            // wait for its 'online' one — which a phone does not always send, so
            // the view sat loading until a reload. Failing instead lets the
            // connectivity probe below bring it back.
            networkMode: 'always',
            retry: 3,
            staleTime: 1000 * 60 * 5,
            gcTime: 1000 * 60 * 30
        },
        mutations: {
            networkMode: 'always',
            retry: false
        }
    },
    queryCache: new QueryCache({
        onError: (error) => {
            // Queries retry 3 times, so this fires only after retries are exhausted.
            if (isConnectivityError(error)) {
                reportNetworkError()
            }
        },
        onSuccess: () => {
            // Any successful response means the backend is reachable.
            dismissBanner()
        }
    }),
    mutationCache: new MutationCache({
        onError: (error) => {
            // Mutations retry: false — show immediately on first network error.
            if (isConnectivityError(error)) {
                reportNetworkError()
            }
        }
    })
})

app.use(VueQueryPlugin, { queryClient })

// While the banner is up, useConnectivity probes the server with this: a /rest
// ping once the client is configured — cheap, and an expired key on it is
// re-minted on the way — or the public version endpoint before that. Any answer
// counts as reachable; only "no answer" keeps the banner up.
setConnectivityProbe(async () => {
    if (subsonicClient.isConfigured()) {
        return (await subsonicClient.checkSession()) !== 'unreachable'
    }
    try {
        await getVersion()
        return true
    } catch (err) {
        return !isNetworkFailure(err)
    }
})

// Back online: whatever failed while the server was unreachable tries again, so
// a view left on an error recovers without a reload.
onReconnect(() => {
    void queryClient.refetchQueries({
        type: 'active',
        predicate: (query) => query.state.status === 'error'
    })
})

// The host settles where the server is — and inside the desktop app how requests
// reach it — before anything renders, so the first /me request already goes to
// the right place. An async function rather than a top-level await, which Vite's
// default build target rejects.
async function bootstrap(): Promise<void> {
    await initHost()
    app.mount('#app')
}

void bootstrap()
