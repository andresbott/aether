<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PlayerLayout from '@/layouts/PlayerLayout.vue'
import SettingsLayout from '@/layouts/SettingsLayout.vue'
import LoginView from '@/views/LoginView.vue'
import ConnectivityBanner from '@/components/layout/ConnectivityBanner.vue'
import { useAuth } from '@/composables/useAuth'
import { subsonicReady } from '@/lib/subsonicSession'

declare module 'vue-router' {
    interface RouteMeta {
        layout?: 'player' | 'settings'
        // Full-bleed main content view: the view owns its own horizontal gutter
        // and self-scrolls, so the shells (DesktopShell / MobileShell) put
        // `main-content--flush` on <main>; the padding it drops lives on
        // `.main-content` in styles/_main.scss.
        flush?: boolean
    }
}

const route = useRoute()

const layouts = {
    player: PlayerLayout,
    settings: SettingsLayout,
} as const

const layout = computed(() => layouts[route.meta.layout ?? 'player'])

// The login gate replaces the whole app, not a route: the SPA bootstraps on
// /me and renders the login view until the server reports an identity (auth
// method "native" only — with "none" nothing is ever gated). While /me is
// still in flight nothing is rendered, so anonymous visitors never see the
// app flash before the login form.
const { isLoading, needsLogin, isAdmin } = useAuth()

// The /settings area is administration only: a non-admin who lands there (a
// typed URL — nothing in their UI links to it) is sent home. Watched rather
// than a router guard because the verdict comes from the async /me bootstrap:
// this fires again when the answer arrives, while a beforeEach would have to
// block navigation on a fetch. isAdmin is false while /me loads, but so is
// isLoading rendering anything — redirect only on a settled non-admin answer.
const router = useRouter()
watchEffect(() => {
    if (isLoading.value || needsLogin.value) return
    if (route.meta.layout === 'settings' && !isAdmin.value) {
        router.replace('/')
    }
})
</script>

<template>
    <ConnectivityBanner />
    <LoginView v-if="needsLogin" />
    <component :is="layout" v-else-if="!isLoading && subsonicReady" />
</template>
