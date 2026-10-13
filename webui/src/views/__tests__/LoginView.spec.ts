import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import PrimeVue from 'primevue/config'
import { explicitLogout, lastServerUrl, sessionExpired } from '@/lib/authState'

const isPending = ref(false)
const mutateAsync = vi.fn()
// The gate's desktop-app states; useAuth keeps both false in the browser.
const needsServer = ref(false)
const needsApiKey = ref(false)
const switchServer = vi.fn()
const serverSaved = vi.fn()
const apiKeySaved = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({
        login: { isPending, mutateAsync },
        needsServer,
        needsApiKey,
        switchServer,
        serverSaved,
        apiKeySaved
    })
}))

const routerReplace = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ replace: routerReplace })
}))

// A browser tab unless a case says otherwise.
const host = vi.hoisted(() => ({
    desktopApp: false,
    setServerUrl: vi.fn<(url: string, kind: 'cookie' | 'apiKey') => Promise<string>>(),
    setSessionKind: vi.fn<(kind: 'cookie' | 'apiKey') => Promise<void>>(),
    setApiKey: vi.fn<(key: string) => Promise<void>>()
}))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl: ref<string | null>(null),
        setServerUrl: host.setServerUrl,
        setSessionKind: host.setSessionKind,
        setApiKey: host.setApiKey,
        httpFetch: vi.fn()
    }
})

// Server.spec covers the probe and the key check themselves; here only their
// verdicts matter. The address rules and the error classes stay real.
const { probeServer, verifyApiKey } = vi.hoisted(() => ({
    probeServer: vi.fn<(address: string) => Promise<unknown>>(),
    verifyApiKey: vi.fn<(url: string, key: string) => Promise<unknown>>()
}))
vi.mock('@/lib/api/Server', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/lib/api/Server')>()),
    probeServer,
    verifyApiKey
}))

import LoginView from '@/views/LoginView.vue'
import { serverUrl } from '@/lib/host'
import {
    AetherAnswerError,
    ApiKeyRejectedError,
    InvalidServerAddressError,
    NotAetherServerError,
    ServerUnreachableError
} from '@/lib/api/Server'

const mountView = () =>
    mount(LoginView, {
        global: { plugins: [PrimeVue] },
        attachTo: document.body
    })

beforeEach(() => {
    host.desktopApp = false
    host.setServerUrl.mockReset()
    host.setSessionKind.mockReset()
    host.setApiKey.mockReset()
    serverUrl.value = null
    lastServerUrl.value = null
    // The expiry flags and the router are shared by every case below, the
    // desktop app's included: left as the last case set them, they would hide a
    // key card's "revoked" line and count an earlier case's redirect.
    sessionExpired.value = false
    explicitLogout.value = false
    routerReplace.mockReset()
    needsServer.value = false
    needsApiKey.value = false
    switchServer.mockReset()
    serverSaved.mockReset()
    apiKeySaved.mockReset()
    probeServer.mockReset()
    verifyApiKey.mockReset()
})

async function fillAndSubmit(w: ReturnType<typeof mountView>, user: string, pw: string) {
    await w.find('#login-username').setValue(user)
    await w.find('#login-password').setValue(pw)
    await w.find('form').trigger('submit')
    await nextTick()
}

describe('LoginView', () => {
    beforeEach(() => {
        sessionExpired.value = false
        explicitLogout.value = false
        isPending.value = false
        mutateAsync.mockReset().mockResolvedValue({ done: true })
        routerReplace.mockReset()
    })

    it('disables the submit button until both fields are filled', async () => {
        const w = mountView()
        const submit = w.find('button[type="submit"]')
        expect(submit.attributes('disabled')).toBeDefined()
        await w.find('#login-username').setValue('alice')
        expect(submit.attributes('disabled')).toBeDefined()
        await w.find('#login-password').setValue('secret')
        expect(submit.attributes('disabled')).toBeUndefined()
    })

    it('submits trimmed username, password and the remember-me flag', async () => {
        const w = mountView()
        await w.find('#login-remember').setValue(true)
        await fillAndSubmit(w, '  alice  ', 'secret')
        expect(mutateAsync).toHaveBeenCalledWith({
            username: 'alice',
            password: 'secret',
            rememberMe: true
        })
    })

    // A fresh session always starts on the landing page — the URL the browser
    // sat on while logged out is deliberately not restored.
    it('redirects to the home page after a successful login', async () => {
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'secret')
        await nextTick()
        expect(routerReplace).toHaveBeenCalledWith('/')
    })

    it('does not navigate when the login fails', async () => {
        mutateAsync.mockRejectedValue({ response: { status: 401 } })
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'wrong')
        await nextTick()
        expect(routerReplace).not.toHaveBeenCalled()
    })

    it('defaults remember-me to off', async () => {
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'secret')
        expect(mutateAsync).toHaveBeenCalledWith(
            expect.objectContaining({ rememberMe: false })
        )
    })

    // The server answers one uniform 401 for every credential-shaped failure,
    // so the view shows one uniform message and clears the password.
    it('shows a credentials error on 401 and clears the password', async () => {
        mutateAsync.mockRejectedValue({ response: { status: 401 } })
        const w = mountView()
        await fillAndSubmit(w, 'alice', 'wrong')
        await nextTick()
        expect(w.find('.login-error').text()).toBe('Wrong username or password.')
        expect((w.find('#login-password').element as HTMLInputElement).value).toBe('')
    })

    it('explains a session expiry when the flag is set', () => {
        sessionExpired.value = true
        expect(mountView().find('.login-expired').text()).toContain('session has expired')
    })

    it('shows no expiry note on a fresh visit', () => {
        expect(mountView().find('.login-expired').exists()).toBe(false)
    })

    // The logout purge resets the query cache, whose refetches 401 and flip
    // sessionExpired — the user asked for this, so the view stays silent.
    it('shows no expiry note after a deliberate logout', () => {
        sessionExpired.value = true
        explicitLogout.value = true
        expect(mountView().find('.login-expired').exists()).toBe(false)
    })
})

describe('LoginView in the browser', () => {
    // The server is the page's own origin: nothing to choose, nothing to show.
    it('shows no server step and no server line', () => {
        serverUrl.value = 'http://localhost:3000'
        const w = mountView()

        expect(w.find('#login-server').exists()).toBe(false)
        expect(w.find('.login-server').exists()).toBe(false)
        expect(w.text()).not.toContain('Server ·')
        expect(w.find('#login-username').exists()).toBe(true)
        expect(w.find('[data-test="use-api-key"]').exists()).toBe(false)
    })
})

describe('LoginView in the desktop app', () => {
    const SERVER = 'http://aether.local:8075'
    const API_KEY_HINT = 'Create one in Aether → User settings → Connected apps.'
    const API_KEY_REVOKED =
        'Your API key was not accepted — it may have been revoked. Enter a new one.'
    const PORTAL_ONLY =
        'This server signs you in through its portal, so only an API key works here.'

    beforeEach(() => {
        host.desktopApp = true
    })

    async function connect(w: ReturnType<typeof mountView>, address: string) {
        await w.find('#login-server').setValue(address)
        await w.find('form').trigger('submit')
        await flushPromises()
    }

    describe('without a server', () => {
        beforeEach(() => {
            needsServer.value = true
        })

        it('asks for the server address and nothing else', () => {
            const w = mountView()

            const field = w.find('#login-server')
            expect(field.exists()).toBe(true)
            expect(field.attributes('placeholder')).toBe('http://aether.local:8075')
            expect(w.find('label[for="login-server"]').text()).toBe('Server address')
            expect(w.find('#login-username').exists()).toBe(false)
            expect(w.find('#login-password').exists()).toBe(false)
            expect(w.find('#login-remember').exists()).toBe(false)
            expect(w.find('button[type="submit"]').text()).toBe('Connect')
        })

        it('enables Connect once an address is typed', async () => {
            const w = mountView()
            const connectButton = w.find('button[type="submit"]')

            await w.find('#login-server').setValue('  ')
            expect(connectButton.attributes('disabled')).toBeDefined()
            await w.find('#login-server').setValue('aether.local:8075')
            expect(connectButton.attributes('disabled')).toBeUndefined()
        })

        // Saving the server is the whole job: the gate then asks its /me and
        // goes on to the app or to this card's credentials by itself.
        it('saves the address the probe settled on, then hands over to the gate', async () => {
            const order: string[] = []
            probeServer.mockResolvedValue({
                kind: 'aether',
                url: SERVER,
                me: { authMethod: 'native' }
            })
            host.setServerUrl.mockImplementation(async (url, kind) => {
                order.push(`saved ${url} as ${kind}`)
                return url
            })
            serverSaved.mockImplementation(() => order.push('gate told'))
            const w = mountView()

            await connect(w, '  aether.local:8075/ ')

            expect(probeServer).toHaveBeenCalledWith('  aether.local:8075/ ')
            expect(order).toEqual([`saved ${SERVER} as cookie`, 'gate told'])
            expect(w.find('.login-error').exists()).toBe(false)
        })

        // A server that redirected http to https is saved on https.
        it('saves the https address the probe moved to', async () => {
            probeServer.mockResolvedValue({
                kind: 'aether',
                url: 'https://aether.example.org',
                me: { authMethod: 'none' }
            })
            host.setServerUrl.mockImplementation(async (url) => url)
            const w = mountView()

            await connect(w, 'aether.example.org')

            expect(host.setServerUrl).toHaveBeenCalledWith('https://aether.example.org', 'cookie')
        })

        it.each([
            ['an unreachable server', ServerUnreachableError, `Could not reach ${SERVER}.`],
            ['no Aether there', NotAetherServerError, `No Aether server answered at ${SERVER}.`]
        ])('reports %s', async (_case, ErrorClass, message) => {
            probeServer.mockRejectedValue(new ErrorClass(SERVER))
            const w = mountView()

            await connect(w, 'aether.local:8075/')

            expect(w.find('.login-error[role="alert"]').text()).toBe(message)
            expect(host.setServerUrl).not.toHaveBeenCalled()
            expect(serverSaved).not.toHaveBeenCalled()
        })

        it("shows the player's own reason when it refuses to save the address", async () => {
            probeServer.mockResolvedValue({
                kind: 'aether',
                url: SERVER,
                me: { authMethod: 'none' }
            })
            host.setServerUrl.mockRejectedValue({
                code: 'Internal',
                message: 'could not save the settings: permission denied'
            })
            const w = mountView()

            await connect(w, SERVER)

            expect(w.find('.login-error').text()).toBe(
                'could not save the settings: permission denied'
            )
            expect(serverSaved).not.toHaveBeenCalled()
        })

        // The plugin's rejection is a bare string naming the request URL.
        it('never shows a transport error as it came', async () => {
            probeServer.mockRejectedValue(`error sending request for url (${SERVER}/api/v0/me)`)
            const w = mountView()

            await connect(w, SERVER)

            expect(w.find('.login-error').text()).toBe('Could not connect to the server.')
        })

        // The probe refuses it before any request (Server.spec); the view only
        // relays why, and never the password typed.
        it('relays why an address with credentials is refused', async () => {
            probeServer.mockRejectedValue(
                new InvalidServerAddressError(
                    'Enter the server address without a username or password.'
                )
            )
            const w = mountView()

            await connect(w, 'http://ada:s3cret@aether.local:8075')

            const error = w.find('.login-error').text()
            expect(error).toBe('Enter the server address without a username or password.')
            expect(host.setServerUrl).not.toHaveBeenCalled()
        })

        // Aether behind an auth portal: only an API key gets in, so the server is
        // saved as a key session and the gate shows the key card.
        it('saves a portal server as a key session', async () => {
            probeServer.mockResolvedValue({ kind: 'portal', url: 'http://aether.example.org' })
            host.setServerUrl.mockImplementation(async (url) => url)
            const w = mountView()

            await connect(w, 'aether.example.org')

            expect(host.setServerUrl).toHaveBeenCalledWith('http://aether.example.org', 'apiKey')
            expect(serverSaved).toHaveBeenCalledTimes(1)
            expect(w.find('.login-notice').exists()).toBe(false)
            expect(w.find('.login-error').exists()).toBe(false)
        })

        it('locks the field and shows progress while connecting', async () => {
            probeServer.mockReturnValue(new Promise(() => {}))
            const w = mountView()

            await connect(w, SERVER)

            expect(w.find('#login-server').attributes('disabled')).toBeDefined()
            const connectButton = w.find('button[type="submit"]')
            expect(connectButton.attributes('disabled')).toBeDefined()
            expect(connectButton.classes()).toContain('p-button-loading')
        })

        it('offers back the address the user switched away from', () => {
            lastServerUrl.value = 'http://old.lan:8075'

            const w = mountView()

            const field = w.find('#login-server').element as HTMLInputElement
            expect(field.value).toBe('http://old.lan:8075')
            expect(w.find('.login-error').exists()).toBe(false)
        })
    })

    // The saved server did not answer /me (down, or moved): the step comes back
    // on that address and says why, instead of a blank window.
    describe('when the saved server does not answer', () => {
        beforeEach(() => {
            serverUrl.value = SERVER
            needsServer.value = true
        })

        it('opens on the saved address, saying it could not be reached', () => {
            const w = mountView()

            expect((w.find('#login-server').element as HTMLInputElement).value).toBe(SERVER)
            expect(w.find('.login-error[role="alert"]').text()).toBe(`Could not reach ${SERVER}.`)
        })

        // Saving the same address changes nothing /me watches; the gate is told,
        // and asks /me again (useAuth.spec, App.serverGate.spec).
        it('connects to the same address again and has the gate ask /me', async () => {
            probeServer.mockResolvedValue({
                kind: 'aether',
                url: SERVER,
                me: { authMethod: 'none' }
            })
            host.setServerUrl.mockImplementation(async (url) => url)
            const w = mountView()

            await w.find('form').trigger('submit')
            await flushPromises()

            expect(host.setServerUrl).toHaveBeenCalledWith(SERVER, 'cookie')
            expect(serverSaved).toHaveBeenCalledTimes(1)
        })

        it('keeps what the user typed when another address fails too', async () => {
            probeServer.mockRejectedValue(new ServerUnreachableError('http://other.lan:8075'))
            const w = mountView()

            await connect(w, 'other.lan:8075')

            expect((w.find('#login-server').element as HTMLInputElement).value).toBe(
                'other.lan:8075'
            )
            expect(w.find('.login-error').text()).toBe('Could not reach http://other.lan:8075.')
        })
    })

    describe('with a server known', () => {
        beforeEach(() => {
            serverUrl.value = SERVER
        })

        it('names the server above the credentials, with a way to change it', async () => {
            const w = mountView()

            const line = w.find('.login-server')
            expect(line.text()).toContain('Server · aether.local:8075')
            expect(w.find('#login-username').exists()).toBe(true)
            expect(w.find('#login-server').exists()).toBe(false)

            await line.find('button').trigger('click')
            expect(switchServer).toHaveBeenCalledTimes(1)
        })

        // switchServer remembers the server it left (useAuth.spec); the step
        // offers it back, also after the gate has remounted the view.
        it('comes back to the server step with the address to correct', async () => {
            switchServer.mockImplementation(async () => {
                lastServerUrl.value = serverUrl.value
                serverUrl.value = null
                needsServer.value = true
            })
            const w = mountView()

            await w.find('.login-server button').trigger('click')
            await nextTick()
            expect((w.find('#login-server').element as HTMLInputElement).value).toBe(SERVER)
            expect(w.find('.login-error').exists()).toBe(false)

            w.unmount()
            const remounted = mountView()
            expect((remounted.find('#login-server').element as HTMLInputElement).value).toBe(SERVER)
        })
    })

    describe('on the key card', () => {
        beforeEach(() => {
            serverUrl.value = SERVER
            needsApiKey.value = true
            host.setApiKey.mockResolvedValue(undefined)
            host.setSessionKind.mockResolvedValue(undefined)
        })

        async function connectWithKey(w: ReturnType<typeof mountView>, key: string) {
            await w.find('#login-api-key').setValue(key)
            await w.find('form').trigger('submit')
            await flushPromises()
        }

        it('asks for an API key, with the server line and the hint, and no password', () => {
            const w = mountView()

            expect(w.find('.login-server').text()).toContain('Server · aether.local:8075')
            expect(w.find('label[for="login-api-key"]').text()).toBe('API key')
            expect(w.find('.login-hint').text()).toBe(API_KEY_HINT)
            expect(w.find('#login-username').exists()).toBe(false)
            expect(w.find('#login-server').exists()).toBe(false)
            expect(w.find('button[type="submit"]').text()).toBe('Connect')
        })

        // Review focus 1: a pasted key often brings whitespace along.
        it('enables Connect only once a key is typed, and trims what it sends', async () => {
            verifyApiKey.mockResolvedValue({ login: 'alice', role: 'user' })
            const w = mountView()
            const connect = w.find('button[type="submit"]')
            expect(connect.attributes('disabled')).toBeDefined()

            await w.find('#login-api-key').setValue('   ')
            expect(connect.attributes('disabled')).toBeDefined()

            await connectWithKey(w, '  aether_k3y \n')
            expect(verifyApiKey).toHaveBeenCalledWith(SERVER, 'aether_k3y')
            expect(host.setApiKey).toHaveBeenCalledWith('aether_k3y')
        })

        it('verifies the key, stores it, seeds the identity and lands on the home page', async () => {
            const order: string[] = []
            verifyApiKey.mockImplementation(async () => {
                order.push('verified')
                return { login: 'alice', role: 'admin' }
            })
            host.setApiKey.mockImplementation(async () => {
                order.push('stored')
            })
            apiKeySaved.mockImplementation(() => order.push('gate told'))
            const w = mountView()

            await connectWithKey(w, 'aether_k3y')

            expect(order).toEqual(['verified', 'gate told', 'stored'])
            expect(apiKeySaved).toHaveBeenCalledWith({ login: 'alice', role: 'admin' })
            expect(routerReplace).toHaveBeenCalledWith('/')
            expect((w.find('#login-api-key').element as HTMLInputElement).value).toBe('')
        })

        it('reports a refused key, clears the field and stores nothing', async () => {
            verifyApiKey.mockRejectedValue(new ApiKeyRejectedError())
            const w = mountView()

            await connectWithKey(w, 'aether_wrong')

            expect(w.find('.login-error[role="alert"]').text()).toBe(
                'This API key was not accepted.'
            )
            expect((w.find('#login-api-key').element as HTMLInputElement).value).toBe('')
            expect(host.setApiKey).not.toHaveBeenCalled()
            expect(apiKeySaved).not.toHaveBeenCalled()
            expect(routerReplace).not.toHaveBeenCalled()
        })

        it.each([
            [
                'an unreachable server',
                new ServerUnreachableError(SERVER),
                `Could not reach ${SERVER}.`
            ],
            [
                'no Aether there',
                new NotAetherServerError(SERVER),
                `No Aether server answered at ${SERVER}.`
            ],
            ["Aether's own error", new AetherAnswerError('internal error'), 'internal error']
        ])('reports %s and keeps the key typed', async (_case, err, message) => {
            verifyApiKey.mockRejectedValue(err)
            const w = mountView()

            await connectWithKey(w, 'aether_k3y')

            expect(w.find('.login-error').text()).toBe(message)
            expect((w.find('#login-api-key').element as HTMLInputElement).value).toBe('aether_k3y')
            expect(host.setApiKey).not.toHaveBeenCalled()
        })

        // The plugin's rejection is a bare string naming the request URL, key included.
        it('never shows a transport error as it came', async () => {
            verifyApiKey.mockRejectedValue(
                `error sending request for url (${SERVER}/rest/getUser.view?apiKey=aether_k3y)`
            )
            const w = mountView()

            await connectWithKey(w, 'aether_k3y')

            expect(w.find('.login-error').text()).toBe('Could not connect to the server.')
        })

        it("shows the player's own reason when it refuses to store the key", async () => {
            verifyApiKey.mockResolvedValue({ login: 'alice', role: 'user' })
            host.setApiKey.mockRejectedValue({
                code: 'Internal',
                message: 'could not save the key: permission denied'
            })
            const w = mountView()

            await connectWithKey(w, 'aether_k3y')

            expect(w.find('.login-error').text()).toBe('could not save the key: permission denied')
            expect(routerReplace).not.toHaveBeenCalled()
        })

        it('explains a revoked key only when the session was lost on its own', () => {
            sessionExpired.value = true
            expect(mountView().find('.login-expired').text()).toBe(API_KEY_REVOKED)

            explicitLogout.value = true
            expect(mountView().find('.login-expired').exists()).toBe(false)
        })

        it('offers the server step through Change', async () => {
            const w = mountView()

            await w.find('.login-server button').trigger('click')

            expect(switchServer).toHaveBeenCalledTimes(1)
        })

        describe('"Sign in with a password instead"', () => {
            const meNative = {
                authMethod: 'native',
                user: null,
                features: { userManagement: true }
            }
            const meProxyAnonymous = {
                authMethod: 'proxy-header',
                user: null,
                features: { userManagement: false }
            }

            it('switches to a cookie session when the server offers a password login', async () => {
                probeServer.mockResolvedValue({ kind: 'aether', url: SERVER, me: meNative })
                const w = mountView()

                await w.find('[data-test="use-password"]').trigger('click')
                await flushPromises()

                expect(probeServer).toHaveBeenCalledWith(SERVER)
                expect(host.setSessionKind).toHaveBeenCalledWith('cookie')
                expect(serverSaved).toHaveBeenCalledTimes(1)
                expect(w.find('.login-notice').exists()).toBe(false)
            })

            it.each([
                ['a portal', { kind: 'portal', url: SERVER }],
                [
                    'a proxy that lets an anonymous /me through',
                    { kind: 'aether', url: SERVER, me: meProxyAnonymous }
                ]
            ])('explains that only a key works on %s', async (_case, answer) => {
                probeServer.mockResolvedValue(answer)
                const w = mountView()

                await w.find('[data-test="use-password"]').trigger('click')
                await flushPromises()

                expect(w.find('.login-notice').text()).toBe(PORTAL_ONLY)
                expect(host.setSessionKind).not.toHaveBeenCalled()
                expect(w.find('#login-api-key').exists()).toBe(true)
            })

            it('reports a server that cannot be reached', async () => {
                probeServer.mockRejectedValue(new ServerUnreachableError(SERVER))
                const w = mountView()

                await w.find('[data-test="use-password"]').trigger('click')
                await flushPromises()

                expect(w.find('.login-error').text()).toBe(`Could not reach ${SERVER}.`)
                expect(host.setSessionKind).not.toHaveBeenCalled()
            })
        })
    })

    describe('on the sign-in card', () => {
        beforeEach(() => {
            serverUrl.value = SERVER
            host.setSessionKind.mockResolvedValue(undefined)
        })

        it('offers an API key instead of a password', async () => {
            const w = mountView()

            await w.find('[data-test="use-api-key"]').trigger('click')
            await flushPromises()

            expect(host.setSessionKind).toHaveBeenCalledWith('apiKey')
        })
    })
})
