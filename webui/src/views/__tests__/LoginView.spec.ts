import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import PrimeVue from 'primevue/config'
import { explicitLogout, lastServerUrl, sessionExpired } from '@/lib/authState'

const isPending = ref(false)
const mutateAsync = vi.fn()
// The gate's desktop-app states; useAuth keeps both false in the browser.
const needsServer = ref(false)
const needsPortal = ref(false)
const switchServer = vi.fn()
const serverSaved = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({
        login: { isPending, mutateAsync },
        needsServer,
        needsPortal,
        switchServer,
        serverSaved
    })
}))

const routerReplace = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ replace: routerReplace })
}))

// A browser tab unless a case says otherwise.
const host = vi.hoisted(() => ({
    desktopApp: false,
    setServerUrl: vi.fn<(url: string) => Promise<string>>()
}))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl: ref<string | null>(null),
        setServerUrl: host.setServerUrl,
        httpFetch: vi.fn()
    }
})

// Server.spec covers the probe itself; here only its verdicts matter. The
// address rules and the error classes stay real.
const probeServer = vi.hoisted(() => vi.fn<(address: string) => Promise<unknown>>())
vi.mock('@/lib/api/Server', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/lib/api/Server')>()),
    probeServer
}))

import LoginView from '@/views/LoginView.vue'
import { serverUrl } from '@/lib/host'
import {
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
    serverUrl.value = null
    lastServerUrl.value = null
    needsServer.value = false
    needsPortal.value = false
    switchServer.mockReset()
    serverSaved.mockReset()
    probeServer.mockReset()
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
    })
})

describe('LoginView in the desktop app', () => {
    const SERVER = 'http://aether.local:8075'
    const PORTAL_MESSAGE =
        'This server signs you in through a web portal, which the desktop app cannot open yet. ' +
        'Open it in a browser, or connect to another server.'

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
            host.setServerUrl.mockImplementation(async (url) => {
                order.push(`saved ${url}`)
                return url
            })
            serverSaved.mockImplementation(() => order.push('gate told'))
            const w = mountView()

            await connect(w, '  aether.local:8075/ ')

            expect(probeServer).toHaveBeenCalledWith('  aether.local:8075/ ')
            expect(order).toEqual([`saved ${SERVER}`, 'gate told'])
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

            expect(host.setServerUrl).toHaveBeenCalledWith('https://aether.example.org')
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

        // Aether behind an auth portal: the app cannot sign in there, so nothing
        // is saved and the user can try another address straight away.
        it('explains a portal-only server and saves nothing', async () => {
            probeServer.mockResolvedValue({ kind: 'portal', url: 'http://aether.example.org' })
            const w = mountView()

            await connect(w, 'aether.example.org')

            expect(w.find('.login-notice').text()).toBe(PORTAL_MESSAGE)
            expect(host.setServerUrl).not.toHaveBeenCalled()
            const field = w.find('#login-server')
            expect(field.attributes('disabled')).toBeUndefined()
            expect((field.element as HTMLInputElement).value).toBe('aether.example.org')
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

            expect(host.setServerUrl).toHaveBeenCalledWith(SERVER)
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

    describe('on a portal-only server', () => {
        beforeEach(() => {
            serverUrl.value = SERVER
            needsPortal.value = true
        })

        it('explains why and offers another server instead of credentials', async () => {
            const w = mountView()

            expect(w.find('.login-notice').text()).toBe(PORTAL_MESSAGE)
            expect(w.find('#login-username').exists()).toBe(false)
            const change = w.find('button')
            expect(change.text()).toBe('Change server')

            await change.trigger('click')
            expect(switchServer).toHaveBeenCalledTimes(1)
        })
    })
})
