import { createApp } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import App from './App.vue'
// @ts-expect-error theme.js has no type declarations
import CustomTheme from '@/theme.js'

import 'primeflex/primeflex.css'
import '@tabler/icons-webfont/dist/tabler-icons-300.min.css'
import '@/assets/style.css'

import PrimeVue from 'primevue/config'

const app = createApp(App)

const applyTheme = () => {
  const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  const htmlElement = document.documentElement

  if (darkModeMediaQuery.matches) {
    htmlElement.classList.add('dark-mode')
  } else {
    htmlElement.classList.remove('dark-mode')
  }
}

applyTheme()
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', applyTheme)

app.use(PrimeVue, {
  theme: {
    preset: CustomTheme,
    options: {
      prefix: 'c',
      darkModeSelector: '.dark-mode',
      cssLayer: false
    }
  }
})

import { createPinia } from 'pinia'
app.use(createPinia())

import router from './router'
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
