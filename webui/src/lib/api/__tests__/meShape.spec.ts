import { describe, it, expect } from 'vitest'
import { isMeResponse } from '@/lib/api/meShape'

// /me is what the gate decides on, in the browser and in the desktop app alike,
// so an answer is Aether's only when it has the whole shape the gate reads.
describe('isMeResponse', () => {
    it.each([
        ['without login', { authMethod: 'none', user: null, features: { userManagement: false } }],
        ['native, nobody signed in', { authMethod: 'native', user: null, features: { userManagement: true } }],
        [
            'native, signed in',
            {
                authMethod: 'native',
                user: { login: 'ada', role: 'admin' },
                features: { userManagement: true }
            }
        ],
        [
            'proxy-header, signed in',
            {
                authMethod: 'proxy-header',
                user: { login: 'ada', role: 'user' },
                features: { userManagement: false }
            }
        ]
    ])("accepts Aether's answer %s", (_case, body) => {
        expect(isMeResponse(body)).toBe(true)
    })

    it.each([
        ['a web page', '<!doctype html><title>Sign in</title>'],
        ['an empty body', undefined],
        ['null', null],
        ['a list', []],
        ['another JSON API', { status: 'ok' }],
        ['an unknown auth method', { authMethod: 'oauth', user: null, features: {} }],
        ['no user field', { authMethod: 'none', features: {} }],
        ['a user without a login', { authMethod: 'native', user: { name: 'ada' }, features: {} }],
        ['a user that is a name', { authMethod: 'native', user: 'ada', features: {} }],
        ['no features', { authMethod: 'none', user: null }],
        ['features that are a list', { authMethod: 'none', user: null, features: [] }]
    ])('refuses %s', (_case, body) => {
        expect(isMeResponse(body)).toBe(false)
    })
})
