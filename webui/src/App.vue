<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PlayerLayout from '@/layouts/PlayerLayout.vue'
import SettingsLayout from '@/layouts/SettingsLayout.vue'
import LoginView from '@/views/LoginView.vue'
import ConnectivityBanner from '@/components/layout/ConnectivityBanner.vue'
import { useAuth } from '@/composables/useAuth'
import { useTrayBridge } from '@/composables/useTrayBridge'
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

// The desktop app's tray menu (Play / Pause, Next) drives the player on every
// layout, so it is wired here, which is always mounted, and not in PlayerLayout,
// which the settings routes unmount while the music plays on.
useTrayBridge()

// The login gate replaces the whole app, not a route: the SPA bootstraps on
// /me — or, in the desktop app's key session, on getUser over /rest — and
// renders the login view until an identity is known (auth method "native"
// only — with "none" nothing is ever gated). While the bootstrap is still in
// flight nothing is rendered, so anonymous visitors never see the app flash
// before the login form. Inside the desktop app the login view also asks for
// the server first, and for an API key where a password cannot sign in.
const { isLoading, needsLogin, needsServer, needsApiKey, isAdmin, managementApi } = useAuth()

// The /settings area is administration only, over /api/v0: a non-admin who
// lands there (a typed URL — nothing in their UI links to it) is sent home, and
// so is an admin whose session has no /api/v0 (a key session). Watched rather
// than a router guard because the verdict comes from the async bootstrap:
// this fires again when the answer arrives, while a beforeEach would have to
// block navigation on a fetch. isAdmin is false while the identity loads, but
// so is isLoading rendering anything — redirect only on a settled answer.
const router = useRouter()
watchEffect(() => {
    if (isLoading.value || needsLogin.value) return
    if (route.meta.layout === 'settings' && !(isAdmin.value && managementApi.value)) {
        router.replace('/')
    }
})
</script>

<template>
    <ConnectivityBanner />
    <LoginView v-if="needsServer || needsLogin || needsApiKey" />
    <component :is="layout" v-else-if="!isLoading && subsonicReady" />
</template>
