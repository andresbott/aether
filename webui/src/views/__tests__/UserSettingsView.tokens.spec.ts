import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, type Ref } from 'vue'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import type { MeUser } from '@/types/users'
import type { ApiToken, CreateTokenResponse } from '@/types/tokens'

const currentUser: Ref<MeUser | null> = ref(null)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ currentUser })
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

const mountView = () =>
    mount(UserSettingsView, {
        global: {
            plugins: [PrimeVue, ToastService],
            stubs: {
                teleport: true
            }
        },
        attachTo: document.body
    })

beforeEach(() => {
    currentUser.value = { login: 'alice', role: 'user' }
    mockTokens.value = []
    vi.clearAllMocks()
})

afterEach(() => {
    currentUser.value = null
    mockTokens.value = []
    useTheme().mode.value = 'auto'
})

describe('UserSettingsView tokens', () => {
    it('creates a usertoken by default and shows the credential pair', async () => {
        const mockResponse: CreateTokenResponse = {
            token: 'aether_abc123defg_S3CRET',
            tokenId: 'abc123defg',
            username: 'abc123defg',
            password: 'S3CRET',
            name: 'phone',
            kind: 'client',
            type: 'usertoken',
            createdAt: new Date().toISOString()
        }

        mockCreateToken.mockImplementation((input: any, options: any) => {
            if (options?.onSuccess) {
                options.onSuccess(mockResponse)
            }
        })

        const wrapper = mountView()
        await wrapper.vm.$nextTick()

        await wrapper.find('#token-name').setValue('phone')
        await wrapper.find('form.token-create').trigger('submit')
        await flushPromises()

        expect(mockCreateToken).toHaveBeenCalledWith(
            expect.objectContaining({ name: 'phone', type: 'usertoken' }),
            expect.anything()
        )

        // Dialog should show username and password
        expect(wrapper.text()).toContain('abc123defg')
        expect(wrapper.text()).toContain('S3CRET')
        expect(wrapper.text()).toContain('Username')
        expect(wrapper.text()).toContain('Password')
    })

    it('creates an apikey when that type is picked and shows the single token', async () => {
        const mockResponse: CreateTokenResponse = {
            token: 'aether_abc123defg_S3CRET',
            tokenId: 'abc123defg',
            name: 'script',
            kind: 'client',
            type: 'apikey',
            createdAt: new Date().toISOString()
        }

        mockCreateToken.mockImplementation((input: any, options: any) => {
            if (options?.onSuccess) {
                options.onSuccess(mockResponse)
            }
        })

        const wrapper = mountView()
        await wrapper.vm.$nextTick()

        // Directly set the token type to apikey
        ;(wrapper.vm as any).newTokenType = 'apikey'
        await wrapper.vm.$nextTick()

        await wrapper.find('#token-name').setValue('script')
        await wrapper.find('form.token-create').trigger('submit')
        await flushPromises()

        expect(mockCreateToken).toHaveBeenCalledWith(
            expect.objectContaining({ type: 'apikey' }),
            expect.anything()
        )
        expect(wrapper.text()).toContain('aether_abc123defg_S3CRET')
    })

    it('shows a type tag on client token rows', async () => {
        mockTokens.value = [
            {
                tokenId: 'a1',
                name: 'phone',
                kind: 'client',
                type: 'usertoken',
                createdAt: new Date().toISOString()
            },
            {
                tokenId: 'b2',
                name: 'script',
                kind: 'client',
                type: 'apikey',
                createdAt: new Date().toISOString()
            }
        ]

        const wrapper = mountView()
        await wrapper.vm.$nextTick()

        expect(wrapper.text()).toContain('App password')
        expect(wrapper.text()).toContain('API key')
    })
})
