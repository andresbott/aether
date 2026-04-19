import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'
import FocusTrap from 'primevue/focustrap'
import StyleClass from 'primevue/styleclass'
import Ripple from 'primevue/ripple'

import CustomTheme from './theme'
import 'primeflex/primeflex.css'
import 'primeicons/primeicons.css'
import './assets/scss/_main.scss'

import App from './App.vue'
import router from './router'
import { subsonicClient } from './lib/api/subsonic'

subsonicClient.initWithDefaults()

const app = createApp(App)

document.documentElement.classList.remove('dark-mode')

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

app.use(createPinia())
app.use(router)

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            refetchOnWindowFocus: false,
            retry: 3,
            staleTime: 1000 * 60 * 5,
            gcTime: 1000 * 60 * 30
        },
        mutations: {
            retry: false
        }
    }
})

app.use(VueQueryPlugin, { queryClient })

app.mount('#app')
