import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive, ref, type Ref } from 'vue'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import type { MeUser } from '@/types/users'
import type { ApiToken } from '@/types/tokens'

// The active tab is a path segment (/user-settings/:tab), so the router mock has
// to behave like a real one: replace() writes the new param back onto the
// reactive route.
const route = reactive({
    params: {} as Record<string, string>,
    query: {} as Record<string, string>
})
const replace = vi.fn((to: { path: string; query: Record<string, string> }) => {
    const tab = to.path.replace(/^\/user-settings\/?/, '')
    route.params = tab ? { tab } : {}
    route.query = to.query
})
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace })
}))

const currentUser: Ref<MeUser | null> = ref(null)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ currentUser, authRequired: ref(false) })
}))

const mockTokens: Ref<ApiToken[]> = ref([])
const mockCreateToken = vi.fn()
const mockRevokeToken = vi.fn()
vi.mock('@/composables/useTokens', () => ({
    useTokens: () => ({ data: mockTokens }),
    useCreateToken: () => ({
        mutate: mockCreateToken,
        isPending: ref(false)
    }),
    useRevokeToken: () => ({
        mutate: mockRevokeToken,
        isPending: ref(false),
        variables: ref(undefined)
    })
}))

import UserSettingsView from '@/views/UserSettingsView.vue'
import { useTheme } from '@/composables/useTheme'

const THEME_CLASSES = ['dark-mode', 'theme-winamp', 'theme-crt']

const mountView = () =>
    mount(UserSettingsView, {
        global: {
            plugins: [PrimeVue, ToastService, ConfirmationService],
            directives: { tooltip: {} },
            stubs: {
                teleport: true
            }
        },
        attachTo: document.body
    })

// useTheme is a module singleton shared with the rest of the suite, so put the
// mode back where it started rather than leaking a hidden theme.
beforeEach(() => {
    useTheme().mode.value = 'auto'
    route.params = {}
    route.query = {}
    replace.mockClear()
})

afterEach(() => {
    useTheme().mode.value = 'auto'
    document.documentElement.classList.remove(...THEME_CLASSES)
})

describe('UserSettingsView', () => {
    it('renders as a main content view with the scaffold title', () => {
        const w = mountView()
        expect(w.find('.content-scaffold').exists()).toBe(true)
        expect(w.find('.scaffold-title h1').text()).toBe('User settings')
    })

    it('shows the signed-in identity in the General panel when a session exists', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        expect(w.find('#panel-general').text()).toContain('Signed in as alice')
        currentUser.value = null
    })

    it('explains the no-login setup when anonymous', () => {
        const w = mountView()
        expect(w.find('#panel-general').text()).toContain('requires no login')
    })

    it('offers only the three standard themes by default', () => {
        const labels = mountView()
            .findAll('.p-selectbutton .p-togglebutton-label')
            .map((b) => b.text())
        expect(labels).toEqual(['Auto', 'Light', 'Dark'])
    })

    // The shortcuts reference moved to the About view.
    it('has no keyboard shortcuts section', () => {
        expect(mountView().text()).not.toContain('Keyboard shortcuts')
    })

    it('lists the hidden themes once they are unlocked', () => {
        useTheme().unlockHiddenThemes()
        const w = mountView()
        const labels = w.findAll('.p-selectbutton .p-togglebutton-label').map((b) => b.text())
        expect(labels).toEqual(['Auto', 'Light', 'Dark', 'Winamp', 'CRT'])
        expect(w.text()).toContain('Nice find')
    })

    it('shows the API tokens section for a logged-in native-mode user', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        expect(w.find('.tokens-section').exists()).toBe(true)
        currentUser.value = null
    })

    it('hides the API tokens section with auth method none', () => {
        currentUser.value = null
        const w = mountView()
        expect(w.find('.tokens-section').exists()).toBe(false)
    })

    it('lists tokens with name and revoke affordance', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        mockTokens.value = [
            {
                tokenId: 't1',
                name: 'Symfonium',
                kind: 'client',
                type: 'apikey',
                createdAt: '2026-01-01T00:00:00Z'
            }
        ]
        const w = mountView()
        expect(w.text()).toContain('Symfonium')
        expect(w.find('.token-revoke').exists()).toBe(true)
        currentUser.value = null
        mockTokens.value = []
    })

    // The revoke control is a trashcan icon button, so its only accessible name
    // comes from aria-label — assert it names the token it would remove.
    it('revokes with a labelled trashcan icon button', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        mockTokens.value = [
            {
                tokenId: 't1',
                name: 'Symfonium',
                kind: 'client',
                type: 'apikey',
                createdAt: '2026-01-01T00:00:00Z'
            }
        ]
        const w = mountView()
        const btn = w.find('.token-revoke')
        expect(btn.text()).toBe('')
        expect(btn.find('.pi-trash').exists()).toBe(true)
        expect(btn.attributes('aria-label')).toBe('Revoke Symfonium')
        currentUser.value = null
        mockTokens.value = []
    })

    it('separates client tokens and first-party app sessions into their own groups', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        mockTokens.value = [
            {
                tokenId: 't1',
                name: 'aether-web',
                kind: 'session',
                type: 'apikey',
                createdAt: '2026-01-01T00:00:00Z'
            },
            {
                tokenId: 't2',
                name: 'Symfonium',
                kind: 'client',
                type: 'apikey',
                createdAt: '2026-01-01T00:00:00Z'
            }
        ]
        const w = mountView()
        const groups = w.findAll('.token-group')
        expect(groups).toHaveLength(2)
        expect(groups[0].find('h3').text()).toBe('Apps & API keys')
        expect(groups[0].text()).toContain('Symfonium')
        expect(groups[0].text()).not.toContain('aether-web')
        expect(groups[1].find('h3').text()).toBe('Your Aether apps')
        expect(groups[1].text()).toContain('aether-web')
        expect(groups[1].text()).not.toContain('Symfonium')
        currentUser.value = null
        mockTokens.value = []
    })

    // One session per first-party app instance: signing in from a second one
    // adds a row rather than replacing the first, and only the row this app
    // minted carries the "This app" tag.
    it('lists one session per signed-in app and tags only this one', async () => {
        const { spaTokenId } = await import('@/lib/subsonicSession')
        currentUser.value = { login: 'alice', role: 'user' }
        spaTokenId.value = 's2'
        mockTokens.value = [
            {
                tokenId: 's1',
                name: 'Chrome on Android',
                kind: 'session',
                type: 'apikey',
                createdAt: '2026-01-01T00:00:00Z'
            },
            {
                tokenId: 's2',
                name: 'Firefox on Linux',
                kind: 'session',
                type: 'apikey',
                createdAt: '2026-01-02T00:00:00Z'
            }
        ]
        const w = mountView()
        const rows = w.findAll('.token-group')[1].findAll('.token-row')
        expect(rows).toHaveLength(2)
        expect(rows[0].text()).toContain('Chrome on Android')
        expect(rows[0].text()).not.toContain('This app')
        expect(rows[1].text()).toContain('Firefox on Linux')
        expect(rows[1].text()).toContain('This app')
        spaTokenId.value = null
        currentUser.value = null
        mockTokens.value = []
    })

    it('offers the two create intents as cards', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        const cards = w.findAll('.intent-card')
        expect(cards).toHaveLength(2)
        expect(cards[0].text()).toContain('Connect a music app')
        expect(cards[1].text()).toContain('Create an API key')
        currentUser.value = null
    })

    it('offers a vertical tablist with the two sections when signed in', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        const tablist = w.find('[role="tablist"]')
        expect(tablist.attributes('aria-orientation')).toBe('vertical')
        expect(w.findAll('[role="tab"]').map((t) => t.text())).toEqual([
            'General',
            'Connected apps'
        ])
        currentUser.value = null
    })

    it('keeps connected apps and first-party app sessions in one tab', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()
        const panel = w.find('#panel-access')
        expect(panel.exists()).toBe(true)
        // Both groups live in the same panel, as they did before the tab split.
        expect(panel.findAll('.token-group').map((g) => g.find('h3').text())).toEqual([
            'Apps & API keys',
            'Your Aether apps'
        ])
        currentUser.value = null
    })

    it('drops the access tab when anonymous, leaving only General', () => {
        currentUser.value = null
        const w = mountView()
        expect(w.findAll('[role="tab"]').map((t) => t.text())).toEqual(['General'])
    })

    it('starts on General and switches panels on tab click', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()

        // v-show keeps every panel mounted, so "shown" means display != none.
        const visible = () =>
            w
                .findAll('[role="tabpanel"]')
                .filter((p) => (p.element as HTMLElement).style.display !== 'none')
                .map((p) => p.attributes('id'))

        expect(visible()).toEqual(['panel-general'])
        expect(w.find('[role="tab"][aria-selected="true"]').text()).toBe('General')

        await w.findAll('[role="tab"]')[1].trigger('click')
        expect(visible()).toEqual(['panel-access'])
        expect(w.find('[role="tab"][aria-selected="true"]').text()).toBe('Connected apps')

        currentUser.value = null
    })

    it('persists the active tab as a path segment', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()

        await w.findAll('[role="tab"]')[1].trigger('click')
        expect(replace).toHaveBeenLastCalledWith({ path: '/user-settings/access', query: {} })

        // General owns the bare path rather than /user-settings/general.
        await w.findAll('[role="tab"]')[0].trigger('click')
        expect(replace).toHaveBeenLastCalledWith({ path: '/user-settings', query: {} })

        currentUser.value = null
    })

    it('opens the tab named in the path', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        route.params = { tab: 'access' }
        const w = mountView()
        expect(w.find('[role="tab"][aria-selected="true"]').text()).toBe('Connected apps')
        currentUser.value = null
    })

    it('falls back to General for an unknown section and rewrites the URL', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        route.params = { tab: 'nope' }
        const w = mountView()
        expect(w.find('[role="tab"][aria-selected="true"]').text()).toBe('General')
        expect(replace).toHaveBeenCalledWith({ path: '/user-settings', query: {} })
        currentUser.value = null
    })

    it('keeps unrelated query params when switching tabs', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        route.query = { keep: 'me' }
        const w = mountView()

        await w.findAll('[role="tab"]')[1].trigger('click')
        expect(replace).toHaveBeenLastCalledWith({
            path: '/user-settings/access',
            query: { keep: 'me' }
        })

        currentUser.value = null
    })

    it('moves between tabs with arrow keys, wrapping at the ends', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()

        const selected = () => w.find('[role="tab"][aria-selected="true"]').text()

        await w.findAll('[role="tab"]')[0].trigger('keydown', { key: 'ArrowDown' })
        expect(selected()).toBe('Connected apps')

        await w.findAll('[role="tab"]')[1].trigger('keydown', { key: 'Home' })
        expect(selected()).toBe('General')

        await w.findAll('[role="tab"]')[0].trigger('keydown', { key: 'End' })
        expect(selected()).toBe('Connected apps')

        // Past the last tab wraps back to the first.
        await w.findAll('[role="tab"]')[1].trigger('keydown', { key: 'ArrowDown' })
        expect(selected()).toBe('General')

        currentUser.value = null
    })

    it('falls back to the first tab when the active one disappears', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()

        await w.findAll('[role="tab"]')[1].trigger('click')
        expect(w.find('[role="tab"][aria-selected="true"]').text()).toBe('Connected apps')

        // Signing out removes the access tab; the panel area must not go blank,
        // and the URL is rewritten back to the default section.
        currentUser.value = null
        await w.vm.$nextTick()
        expect(w.find('[role="tab"][aria-selected="true"]').text()).toBe('General')
        expect(replace).toHaveBeenLastCalledWith({ path: '/user-settings', query: {} })
    })

    it('shows the plaintext exactly once after creation', async () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mountView()

        // The create form lives in a dialog behind the API-key intent card.
        await w.findAll('.intent-card')[1].trigger('click')
        await w.vm.$nextTick()

        const nameInput = w.find('#token-name')
        await nameInput.setValue('MyToken')

        // Simulate successful creation
        mockCreateToken.mockImplementation((input: any, options: any) => {
            if (options?.onSuccess) {
                options.onSuccess({
                    tokenId: 't2',
                    name: 'MyToken',
                    kind: 'client',
                    type: 'apikey',
                    createdAt: '2026-01-02T00:00:00Z',
                    token: 'aether_x_y'
                })
            }
        })

        await w.find('.token-create').trigger('submit')
        await w.vm.$nextTick()
        await w.vm.$nextTick() // Extra tick for dialog visibility

        const plaintextEl = w.find('.token-plaintext')
        expect(plaintextEl.exists()).toBe(true)
        expect(plaintextEl.text()).toContain('aether_x_y')
        currentUser.value = null
    })
})
